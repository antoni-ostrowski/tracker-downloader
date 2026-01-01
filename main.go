package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

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
}

func main() {
	log.Print("starting")

	tracksFile, err := os.OpenFile("/Users/antoni-ostrowski/Desktop/yeat-tracker.csv", os.O_RDWR|os.O_CREATE, os.ModePerm)
	if err != nil {
		log.Fatal("failed to open csv file")
		panic(err)
	}

	log.Print("opened csv file")

	defer tracksFile.Close()

	allRows := []*Track{}

	if err := gocsv.UnmarshalFile(tracksFile, &allRows); err != nil { // Load clients from file
		log.Fatalf("Failed to unmarshal %v", err)
		panic(err)
	}

	for _, track := range allRows {
		if strings.EqualFold(strings.TrimSpace(track.Links), "Source Needed") {
			continue
		}

		links := strings.Fields(track.Links)

		if len(links) == 0 {
			continue
		}

		fmt.Printf("Found %d links\n", len(links))
		for i, link := range links {
			fmt.Printf("%d: %s\n", i+1, link)
		}

		track.Name = strings.ReplaceAll(track.Name, "\n", " ")

		data, _ := json.MarshalIndent(track, "", "  ")
		fmt.Println(string(data))
		fmt.Printf("\n")
	}

}
