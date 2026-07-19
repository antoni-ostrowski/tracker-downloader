package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path"
	"slices"
	"strings"
	"sync"

	"github.com/gocarina/gocsv"
	"go.senan.xyz/taglib"
)

type Track struct {
	Era            string `csv:"Era"`
	Name           string `csv:"Name"`
	Notes          string `csv:"Notes\n(Join the Yeat Hub Discord!)"`
	FileDate       string `csv:"File Date"`
	Type           string `csv:"Type"`
	AvailableLen   string `csv:"Available Length"`
	Quality        string `csv:"Quality"`
	Links          string `csv:"Link(s)"`
	FirstPreview   string `csv:"First Preview"`
	LeakDate       string `csv:"Leak Date"`
	OGFileLeakDate string `csv:"OG File Leak Date"`
	RealLinks      []string
	OutputFilePath string
}

func main() {

	outputDirPtr := flag.String("o", "", "output: directory for downloaded files")
	trackerPathPtr := flag.String("i", "", "input: tracker csv file path")
	workerCountPtr := flag.Int("w", 5, "opt: worker count")
	baseCoverPathPtr := flag.String("c", "", "opt: cover dir path")

	flag.Parse()

	outputDir := *outputDirPtr
	trackerPath := *trackerPathPtr
	workerCount := *workerCountPtr
	baseCoverPath := *baseCoverPathPtr

	log.Printf("Starting downloader. Output directory set to: %s\n tracker path set to: %v\n worker count set to: %v\n", outputDir, trackerPath, workerCount)

	err := os.MkdirAll(outputDir, 0755)
	if err != nil {
		log.Fatalf("Failed to create output dir %v", err)
	}

	tracksFile, err := os.OpenFile(trackerPath, os.O_RDWR|os.O_CREATE, os.ModePerm)
	if err != nil {
		log.Fatal("failed to open csv file")
		panic(err)
	}

	log.Print("opened csv file")

	defer tracksFile.Close()

	allRows := []Track{}

	if err := gocsv.UnmarshalFile(tracksFile, &allRows); err != nil {
		log.Fatalf("Failed to unmarshal %v", err)
		panic(err)
	}

	log.Println("track amount: ", len(allRows))

	tracksCh := make(chan Track)
	var processWg sync.WaitGroup

	// start workers
	for i := range workerCount {
		processWg.Add(1)

		go func(id int) {
			defer processWg.Done()

			for track := range tracksCh {
				log.Printf("[WORKER %v] processing %v \n", id, track.Name)

				for _, link := range track.RealLinks {

					if _, err := os.Stat(track.OutputFilePath); err == nil {
						log.Printf("[WORKER %v] File %s already exists, skipping...", id, track.Name)
						continue
					}

					downloadLink := createDownloadUrl(link)
					if len(downloadLink) == 0 {
						log.Printf("[WORKER %v] No download link found", id)
						continue
					}

					log.Printf("[WORKER %v] attempting to download %v \n", id, downloadLink)

					finalName, err := downloadFile(downloadLink, track, outputDir)
					if err != nil {
						log.Printf("Failed to download file %v \n", err)
						continue
					}

					err = taglib.WriteTags(finalName, map[string][]string{
						taglib.Album:     {track.Era},
						taglib.Title:     {track.Name},
						taglib.Artist:    {"yeat"},
						"Notes":          {track.Notes},
						"FileDate":       {track.FileDate},
						"AvailableLen":   {track.AvailableLen},
						"Quality":        {track.Quality},
						"FirstPreview":   {track.FirstPreview},
						"LeakDate":       {track.LeakDate},
						"OGFileLeakDate": {track.OGFileLeakDate},
					}, 0)

					if err != nil {
						log.Printf("Failed to write metadata %v \n", err)
						continue
					}

					imageBytes := getImageForTrack(track, baseCoverPath)

					err = taglib.WriteImage(finalName, imageBytes)

					if err != nil {
						log.Printf("Failed to embeed image %v \n", err)
						continue
					}

					log.Printf("[WORKER %v] successfully downloaded %v \n", id, track.Name)

				}

			}

		}(i)

	}

	// filter junk, then feed the channel
	slices.Reverse(allRows)
	for _, track := range allRows {

		links := getTracksLinks(track)
		if len(links) == 0 {
			continue
		}

		track.OutputFilePath = path.Join(outputDir, track.Name+".mp3")

		track.RealLinks = links

		track.Name = strings.ReplaceAll(track.Name, "\n", " ")

		trackCopy := track
		tracksCh <- trackCopy

	}

	close(tracksCh)

	processWg.Wait()

}

func createDownloadUrl(link string) string {
	var trackId string
	if len(link) >= 32 {
		trackId = link[len(link)-32:]
	} else {
		return ""
	}

	const baseApiUrl = "https://api.pillows.su"
	downloadLink := baseApiUrl + "/api/download/" + trackId
	return downloadLink
}

func getTracksLinks(track Track) []string {
	if strings.EqualFold(strings.TrimSpace(track.Links), "Source Needed") {
		return []string{}
	}

	links := strings.Fields(track.Links)
	links = slices.DeleteFunc(links, func(s string) bool {
		lowerS := strings.ToLower(strings.TrimSpace(s))

		// 1. If it's NOT from pillows.su, delete it.
		if !strings.Contains(lowerS, "pillows.su") {
			return true
		}

		// 2. If it explicitly ends in .jpg, delete it.
		if strings.HasSuffix(lowerS, ".jpg") {
			return true
		}

		// Otherwise, keep it (these are your /api/download/ID links)
		return false
	})

	return links
}

func downloadFile(downloadLink string, track Track, outputDir string) (string, error) {
	resp, err := http.Get(downloadLink)
	if err != nil {
		return "", errors.New("Failed to request the download link %v")
	}

	defer resp.Body.Close()

	ext := ".mp3"
	contentType := resp.Header.Get("Content-Type")

	if strings.Contains(contentType, "video/mp4") || strings.Contains(contentType, "audio/mp4") {
		ext = ".mp4"
	} else if strings.Contains(contentType, "audio/x-m4a") || strings.Contains(contentType, "audio/m4a") {
		ext = ".m4a"
	} else if strings.Contains(contentType, "audio/wav") || strings.Contains(contentType, "audio/x-wav") {
		ext = ".wav"
	} else if strings.Contains(contentType, "audio/flac") || strings.Contains(contentType, "audio/x-flac") {
		ext = ".flac"
	} else if strings.Contains(contentType, "audio/mpeg") {
		ext = ".mp3"
	} else if strings.Contains(contentType, "audio/ogg") {
		ext = ".ogg"
	}

	finalName := path.Join(outputDir, track.Name+ext)
	fmt.Printf("Saving as:%v\n", finalName)

	outFile, err := os.Create(finalName)
	if err != nil {
		return "", errors.New("Failed to create out file %v")
	}
	defer outFile.Close()

	_, err = io.Copy(outFile, resp.Body)
	if err != nil {
		return "", errors.New("Failed to copy the file from body to out file somehow %v")
	}

	if strings.HasSuffix(finalName, ".mp4") {
		err := processVideoToAudio(finalName)
		if err == nil {
			finalName = strings.TrimSuffix(finalName, ".mp4") + ".mp3"
		} else {
			fmt.Println("Error:", err)
		}
	}

	return finalName, nil

}

func getImageForTrack(track Track, base string) []byte {
	era := strings.TrimSpace(track.Era)
	imagePath := path.Join(base, era+".jpg")

	imgData, err := os.ReadFile(imagePath)
	if err != nil {
		imgData, err = os.ReadFile(path.Join(base, "default.jpg"))
		if err != nil {
			return []byte{}
		}
	}

	return imgData
}

func processVideoToAudio(mp4Path string) error {
	// 1. Create the new filename by replacing .mp4 with .mp3
	mp3Path := strings.TrimSuffix(mp4Path, ".mp4") + ".mp3"

	// 2. Run FFmpeg
	// -i: input
	// -vn: no video
	// -y: overwrite mp3 if it already exists
	cmd := exec.Command("ffmpeg", "-i", mp4Path, "-vn", "-ar", "44100", "-ac", "2", "-b:a", "192k", "-y", mp3Path)

	fmt.Printf("Converting %s to MP3...\n", mp4Path)
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("conversion failed: %v", err)
	}

	// 3. Delete the original MP4 file to "replace" it
	err = os.Remove(mp4Path)
	if err != nil {
		return fmt.Errorf("could not delete original mp4: %v", err)
	}

	fmt.Println("Success! File replaced with MP3.")
	return nil
}
