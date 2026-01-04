package main

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/gocarina/gocsv"
	"go.senan.xyz/taglib"
)

type Track struct {
	Era            string `csv:"Era"`
	Name           string `csv:"Name\n(Check out the Tracker website!)"`
	Notes          string `csv:"Notes\n(Join the Yeat Tracker Discord!)"`
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
	if len(os.Args) < 5 {
		fmt.Println("Usage: ./my-program <output-directory> <tracker-csv-path> <worker-count>")
		fmt.Println("Example: ./my-program ./test-music /Users/antoni-ostrowski/Desktop/yeat-tracker.csv 20")
		return
	}

	outputDir := os.Args[1]
	trackerPath := os.Args[2]
	workerCount, workerCountErr := strconv.Atoi(os.Args[3])
	baseCoverPath := os.Args[4]
	if workerCountErr != nil {
		log.Println("Invalid worker count integer")
		return
	}

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

		tracksCh <- track
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
	} else if strings.Contains(contentType, "audio/wav") {
		ext = ".wav"
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

	return finalName, nil

}

func getImageForTrack(track Track, base string) []byte {
	var imagePath string

	era := strings.TrimSpace(track.Era)

	switch {
	case strings.Contains(era, "530"):
		imagePath = base + "530.png"

	case strings.Contains(era, "1500"):
		imagePath = base + "1500.jpg"

	case strings.Contains(era, "Super Sonic"):
		imagePath = base + "super-sonic.jpg"

	case strings.Contains(era, "Deep Blue $trips"):
		imagePath = base + "deep-blue-strips.jpg"

	case strings.Contains(era, "Wake Up Call"):
		imagePath = base + "wake-up-call.jpg"

	case strings.Contains(era, "Elegance"):
		imagePath = base + "elegance.jpg"

	case strings.Contains(era, "Different Creature"):
		imagePath = base + "diff-creature.png"

	case strings.Contains(era, "I'm So Me"):
		imagePath = base + "im-so-me.png"

	case strings.Contains(era, "We Us"):
		imagePath = base + "we-us.jpg"

	case strings.Contains(era, "DC2"):
		imagePath = base + "diff-creature-2.jpg"

	case strings.Contains(era, "Hold Ön"):
		imagePath = base + "hold-on.jpg"

	case strings.Contains(era, "Alivë"):
		imagePath = base + "alive.png"

	case strings.Contains(era, "4L with us"):
		imagePath = base + "4l-with-us.png"

	case strings.Contains(era, "4L"):
		imagePath = base + "4l.png"

	case strings.Contains(era, "Up 2 Më [V1]"):
		imagePath = base + "up2me1.jpg"

	case strings.Contains(era, "Trëndi"):
		imagePath = base + "trendi.png"

	case strings.Contains(era, "Up 2 Më [V3]"):
		imagePath = base + "up2me3.png"

	case strings.Contains(era, "2 Alivë"):
		imagePath = base + "2alive.jpg"

	case strings.Contains(era, "Super geëky"):
		imagePath = base + "super-geeky.jpg"

	case strings.Contains(era, "2 Alivë (Geëk Pack)"):
		imagePath = base + "2alive-geep-pack.jpg"

	case strings.Contains(era, "Lyfë"):
		imagePath = base + "lyfe.jpg"

	case strings.Contains(era, "AftërLyfe"):
		imagePath = base + "afterlyfe.jpg"

	case strings.Contains(era, "AftërLyfe (Deluxe)"):
		imagePath = base + "afterlyfe-deluxe.jpg"

	case strings.Contains(era, "Lyfëstyle [V1]"):
		imagePath = base + "lyfestyle1.jpg"

	case strings.Contains(era, "2093"):
		imagePath = base + "2093.png"

	case strings.Contains(era, "LYFESTYLE [V2]"):
		imagePath = base + "lyfestyle2.png"

	case strings.Contains(era, "A DANGEROUS LYFE [V1]"):
		imagePath = base + "adl.jpg"

	case strings.Contains(era, "LYFESTYLE DIGITAL DELUXE"):
		imagePath = base + "lyfestyle2-deluxe.png"

	case strings.Contains(era, "A DANGEROUS LYFE [V2]"):
		imagePath = base + "adl.jpg"

	case strings.Contains(era, "DANGEROUS SUMMER"):
		imagePath = base + "ds.jpg"

	case strings.Contains(era, "A DANGEROUS LYFE [V3]"):
		imagePath = base + "adl.jpg"

	case strings.Contains(era, "A DANGEROUS LYFE [V4]"):
		imagePath = base + "adl.jpg"

	default:
		imagePath = "assets/images/eras/default_yeat_tracker_cover.jpg"
	}

	// Reading the image file from the local path
	imgData, err := os.ReadFile(imagePath)
	if err != nil {
		log.Printf("Warning: Could not read image for era %s at path %s: %v\n", era, imagePath, err)
		return []byte{}
	}

	return imgData
}
