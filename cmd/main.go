package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"regexp"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt"
	"golang.org/x/net/websocket"
)

type Event struct {
	Reg string `json:"Reg"`
}

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

var (
	cameraImage []byte
	mutex       sync.Mutex
)

func main() {
	apiUrlPTR := flag.String("apiUrl", "", "URL of API endpoint")
	useRaspiPTR := flag.Bool("raspi", false, "Use Raspberry Pi camera (raspivid) as video source")
	webServerPTR := flag.Bool("webserver", false, "Start a web server to display camera output")
	cameraURLPTR := flag.String("cameraURL", "", "URL of the IP camera's RTSP stream")
	flag.Parse()
	if *webServerPTR {
		go func() {
			startWebServer()
		}()
	}
	done := make(chan bool)
	go func() {
		for {
			if *useRaspiPTR {
				captureFromRaspi(*apiUrlPTR)
			} else if *cameraURLPTR != "" {
				captureFromIPCamera(*apiUrlPTR, *cameraURLPTR)
			}
		}
	}()
	<-done
}

func startWebServer() {
	router := gin.Default()
	router.GET("/camera", func(c *gin.Context) {
		mutex.Lock()
		defer mutex.Unlock()
		if cameraImage != nil {
			c.Header("Content-Type", "image/jpeg")
			c.Data(http.StatusOK, "image/jpeg", cameraImage)
		} else {
			c.String(http.StatusNotFound, "Camera image not available")
		}
	})
	router.Run(":8080")
}

func captureFromIPCamera(apiUrl string, cameraURL string) {
	wsURL := "ws://" + cameraURL
	ws, err := websocket.Dial(wsURL, "", "http://localhost")
	if err != nil {
		log.Println("Error connecting to camera WebSocket:", err)
		return
	}
	defer ws.Close()
	for {
		frame := make([]byte, 1024*1024)
		n, err := ws.Read(frame)
		if err != nil {
			log.Println("Error reading camera frame:", err)
			return
		}
		processFrameWithOpenALPR(frame[:n], apiUrl)
	}
}

func captureFromRaspi(apiUrl string) {
	libcameraCmd := exec.Command("libcamera-jpeg", "--output", "-")
	var frameOutput bytes.Buffer
	libcameraCmd.Stdout = &frameOutput
	err := libcameraCmd.Run()
	if err != nil {
		log.Println("Error capturing image with libcamera-jpeg:", err)
		return
	}
	mutex.Lock()
	defer mutex.Unlock()
	cameraImage = frameOutput.Bytes()
	processFrameWithOpenALPR(frameOutput.Bytes(), apiUrl)
}

func processFrameWithOpenALPR(frame []byte, apiUrl string) {

	openalprCmd := exec.Command("alpr", "-c", "gb", "-j", "-")
	openalprCmd.Stdin = bytes.NewReader(frame)
	var openalprOutput bytes.Buffer
	openalprCmd.Stdout = &openalprOutput
	err := openalprCmd.Run()
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
				fmt.Printf("Will send license plate - %v\n", result.Plate)
				sendPlate(result.Plate, apiUrl)
			} else {
				fmt.Printf("Plate detected but not a valid one - %v\n", result.Plate)
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

func sendPlate(plate, apiUrl string) {
	data := Event{
		Reg: plate,
	}
	jsonData, err := json.Marshal(data)
	if err != nil {
		log.Println("Error encoding JSON:", err)
		return
	}
	if apiUrl == "" {
		log.Println("Would have sent ", plate, " but no apiUrl was set.")
	}
	req, err := http.NewRequest("POST", apiUrl, bytes.NewBuffer(jsonData))
	if err != nil {
		log.Println("Error creating request for API:", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	tok, err := GenerateToken()
	if err != nil {
		log.Println("Error generating auth token:", err)
	}
	req.Header.Set("Authorization", tok)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Println("Error sending request to API:", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		log.Println("Successfully sent to API:", plate)
	} else {
		log.Println("Request failed with code:", resp.StatusCode)
	}
}

func GenerateToken() (string, error) {
	token := jwt.New(jwt.SigningMethodHS256)
	claims := token.Claims.(jwt.MapClaims)
	claims["exp"] = time.Now().Add(time.Hour * 24).Unix()
	claims["sub"] = "user123"
	tokenString, err := token.SignedString([]byte("secretkey"))
	if err != nil {
		return "", err
	}
	return tokenString, nil
}
