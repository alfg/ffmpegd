package main

import (
	"fmt"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/websocket"
)

const (
	ProgressInterval = time.Second * 1
	Logo             = `
███████╗███████╗███╗   ███╗██████╗ ███████╗ ██████╗ ██████╗ 
██╔════╝██╔════╝████╗ ████║██╔══██╗██╔════╝██╔════╝ ██╔══██╗
█████╗  █████╗  ██╔████╔██║██████╔╝█████╗  ██║  ███╗██║  ██║
██╔══╝  ██╔══╝  ██║╚██╔╝██║██╔═══╝ ██╔══╝  ██║   ██║██║  ██║
██║     ██║     ██║ ╚═╝ ██║██║     ███████╗╚██████╔╝██████╔╝
╚═╝     ╚═╝     ╚═╝     ╚═╝╚═╝     ╚══════╝ ╚═════╝ ╚═════╝ 
	`
	Banner = "[\u001b[32mffmpegd\u001b[0m] - websocket server for ffmpeg-commander.\n"
	Usage  = `
Usage:
	ffmpegd [port]
	ffmpegd version -- This version.
	`

	Version = "0.0.1"
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
	printBanner()

	startServer()
}

func printBanner() {
	fmt.Println(Logo)
	fmt.Println(Banner)
}

func startServer() {
	http.HandleFunc("/ws", handleConnections)
	go handleMessages()

	fmt.Println("Server started on port :8080.")
	fmt.Println("Go to \u001b[33mhttps://alfg.github.io/ffmpeg-commander\u001b[0m to connect!")
	fmt.Println("")
	fmt.Printf("Waiting for connection...")
	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		fmt.Println("ListenAndServe: ", err)
	}
}

func handleConnections(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Println(err)
	}
	defer ws.Close()

	// Register client.
	clients[ws] = true

	for {
		fmt.Printf("\rWaiting for connection......\u001b[32mconnected!\u001b[0m")
		var msg Message
		// Read in a new message as JSON and map it to a Message object.
		err := ws.ReadJSON(&msg)
		if err != nil {
			fmt.Printf("\rWaiting for connection...\u001b[31mdisconnected!\u001b[0m")
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
	probe := FFProbe{}
	probeData := probe.Run(input)

	ffmpeg := &FFmpeg{}
	go trackEncodeProgress(probeData, ffmpeg)
	err := ffmpeg.Run(input, output, payload)
	if err != nil {
		close(progressCh)
		panic(err)
	}
	close(progressCh)

	for client := range clients {
		p := &Status{
			Percent: 100,
		}
		err := client.WriteJSON(p)
		if err != nil {
			fmt.Println("error: %w", err)
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
			fmt.Printf("\rWaiting for next job...                                                    ")
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

				fmt.Printf("\rEncoding... %d / %d (%0.2f%%) %s @ %0.2f", currentFrame, totalFrames, pct, speed, fps)

				for client := range clients {
					p := &Status{
						Percent: pct,
						Speed:   speed,
						FPS:     fps,
					}
					err := client.WriteJSON(p)
					if err != nil {
						fmt.Println("error: %w", err)
						client.Close()
						delete(clients, client)
					}
				}
			}
		}
	}
}
