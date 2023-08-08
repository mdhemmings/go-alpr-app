package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"regexp"
	"strings"
)

type OpenALPROutput struct {
	Results []struct {
		Plate            string  `json:"plate"`
		Confidence       float32 `json:"confidence"`
		MatchesTemplate  int     `json:"matches_template"`
		PlateIndex       int     `json:"plate_index"`
		Region           string  `json:"region"`
		RegionConfidence int     `json:"region_confidence"`
		ProcessingTimeMs float32 `json:"processing_time_ms"`
		RequestedTopN    int     `json:"requested_topn"`
		Coordinates      []struct {
			X int `json:"x"`
			Y int `json:"y"`
		} `json:"coordinates"`
		Candidates []struct {
			Plate           string  `json:"plate"`
			Confidence      float32 `json:"confidence"`
			MatchesTemplate int     `json:"matches_template"`
		} `json:"candidates"`
	} `json:"results"`
}

func main() {
	for {
		cmd := exec.Command("ffmpeg", "-i", "rtsp://127.0.0.1:554/stream", "-vframes", "1", "-f", "image2pipe", "-")
		var frameOutput bytes.Buffer
		cmd.Stdout = &frameOutput
		err := cmd.Run()
		if err != nil {
			log.Println("Error capturing frame:", err)
			return
		}

		openalprCmd := exec.Command("alpr", "-c", "gb", "-j", "-")
		openalprCmd.Stdin = strings.NewReader(frameOutput.String())
		var openalprOutput bytes.Buffer
		openalprCmd.Stdout = &openalprOutput
		err = openalprCmd.Run()
		if err != nil {
			log.Println("Error running OpenALPR:", err)
			return
		}

		var alprData OpenALPROutput
		err = json.Unmarshal(openalprOutput.Bytes(), &alprData)
		if err != nil {
			log.Println("Error parsing OpenALPR JSON:", err)
			return
		}

		for _, result := range alprData.Results {
			fmt.Printf("License Plate: %v, Confidence: %v\n", result.Plate, result.Confidence)
			if result.Confidence > 80 {
				match := isValidPlate(result.Plate)
				if match {
					fmt.Printf("Would have sent licence plate - %v\n", result.Plate)
				} else {
					fmt.Printf("Plate detected but not a valid one - %v\n", result.Plate)
				}
			}
		}
	}
}

func isValidPlate(plate string) (valid bool) {
	validUKPlate := "(^[A-Z]{2}[0-9]{2}[A-Z]{3}$)|(^[A-Z][0-9]{1,3}[A-Z]{3}$)|(^[A-Z]{3}[0-9]{1,3}[A-Z]$)|(^[0-9]{1,4}[A-Z]{1,2}$)|(^[0-9]{1,3}[A-Z]{1,3}$)|(^[A-Z]{1,2}[0-9]{1,4}$)|(^[A-Z]{1,3}[0-9]{1,3}$)"
	re, err := regexp.Compile(validUKPlate)
	if err != nil {
		fmt.Println("Error compiling regexp:", err)
	}
	valid = re.MatchString(plate)
	return
}
