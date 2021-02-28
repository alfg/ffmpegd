package main

import (
	"fmt"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"
)

const (
	// ProgressInterval ffmpeg progress.
	ProgressInterval = time.Second * 2
)

var clients = make(map[*websocket.Conn]bool)
var broadcast = make(chan Message)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// Message payload from client.
type Message struct {
	Type    string `json:"type"`
	Input   string `json:"input"`
	Output  string `json:"output"`
	Payload string `json:"payload"`
}

// Status response to client.
type Status struct {
	Percent float64 `json:"percent"`
	Speed   string  `json:"speed"`
	FPS     float64 `json:"fps"`
}

var progressCh chan struct{}

func main() {
	fmt.Println("ffmpegd")

	http.HandleFunc("/ws", handleConnections)

	go handleMessages()

	log.Println("http server started on :8080")
	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}

}

func handleConnections(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer ws.Close()

	// Register client.
	clients[ws] = true

	for {
		var msg Message
		// Read in a new message as JSON and map it to a Message object.
		err := ws.ReadJSON(&msg)
		if err != nil {
			log.Printf("error: %v", err)
			delete(clients, ws)
			break
		}
		// Send the newly received message to the broadcast channel.
		broadcast <- msg
	}

}

func handleMessages() {
	for {
		msg := <-broadcast

		if msg.Type == "encode" {
			runEncode(msg.Input, msg.Output, msg.Payload)
		}
	}
}

func runEncode(input, output, payload string) {
	input = "tears-of-steel-720p.mp4"
	output = "out.mp4"

	probe := FFProbe{}
	probeData := probe.Run(input)
	fmt.Println(probeData)

	ffmpeg := &FFmpeg{}
	go trackEncodeProgress(probeData, ffmpeg)
	err := ffmpeg.Run(input, output, payload)
	if err != nil {
		close(progressCh)
		panic(err)
	}
	close(progressCh)
	fmt.Println(ffmpeg)

	for client := range clients {
		p := &Status{
			Percent: 100,
		}
		err := client.WriteJSON(p)
		if err != nil {
			log.Printf("error: %w", err)
			client.Close()
			delete(clients, client)
		}
	}
}

func trackEncodeProgress(p *FFProbeResponse, f *FFmpeg) {
	progressCh = make(chan struct{})
	ticker := time.NewTicker(ProgressInterval)

	for {
		select {
		case <-progressCh:
			ticker.Stop()
			return
		case <-ticker.C:
			currentFrame := f.Progress.Frame
			totalFrames, _ := strconv.Atoi(p.Streams[0].NbFrames)
			speed := f.Progress.Speed
			fps := f.Progress.FPS

			// Only track progress if we know the total frames.
			if totalFrames != 0 {
				pct := (float64(currentFrame) / float64(totalFrames)) * 100
				pct = math.Round(pct*100) / 100

				log.Infof("progress: %d / %d - %0.2f%%", currentFrame, totalFrames, pct)
				log.Info(speed, fps)

				for client := range clients {
					p := &Status{
						Percent: pct,
						Speed:   speed,
						FPS:     fps,
					}
					fmt.Println(p)
					err := client.WriteJSON(p)
					if err != nil {
						log.Printf("error: %w", err)
						client.Close()
						delete(clients, client)
					}
				}
			}
		}
	}
}
