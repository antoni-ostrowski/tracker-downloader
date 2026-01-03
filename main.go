package main

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"

	"github.com/gocarina/gocsv"
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
	if len(os.Args) < 4 {
		fmt.Println("Usage: ./my-program <output-directory> <tracker-csv-path> <worker-count>")
		fmt.Println("Example: ./my-program ./test-music /Users/antoni-ostrowski/Desktop/yeat-tracker.csv 20")
		return
	}

	outputDir := os.Args[1]
	trackerPath := os.Args[2]
	workerCount, workerCountErr := strconv.Atoi(os.Args[3])
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

					err := downloadFile(downloadLink, track.OutputFilePath)
					if err != nil {
						log.Printf("Failed to download file %v", err)
						continue
					}

					log.Printf("[WORKER %v] successfully downloaded %v \n", id, track.Name)

				}

			}

		}(i)

	}

	// filter junk, then feed the channel
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

	return links
}

func downloadFile(downloadLink string, filePath string) error {

	resp, err := http.Get(downloadLink)
	if err != nil {
		return errors.New("Failed to request the download link %v")
	}

	defer resp.Body.Close()

	outFile, err := os.Create(filePath)
	if err != nil {
		return errors.New("Failed to create out file %v")
	}
	defer outFile.Close()

	_, err = io.Copy(outFile, resp.Body)
	if err != nil {
		return errors.New("Failed to copy the file from body to out file somehow %v")
	}

	return nil

}
