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
	logo = `
███████╗███████╗███╗   ███╗██████╗ ███████╗ ██████╗ ██████╗ 
██╔════╝██╔════╝████╗ ████║██╔══██╗██╔════╝██╔════╝ ██╔══██╗
█████╗  █████╗  ██╔████╔██║██████╔╝█████╗  ██║  ███╗██║  ██║
██╔══╝  ██╔══╝  ██║╚██╔╝██║██╔═══╝ ██╔══╝  ██║   ██║██║  ██║
██║     ██║     ██║ ╚═╝ ██║██║     ███████╗╚██████╔╝██████╔╝
╚═╝     ╚═╝     ╚═╝     ╚═╝╚═╝     ╚══════╝ ╚═════╝ ╚═════╝ 
                                                      v0.0.1
	`
	version     = "0.0.1"
	description = "[\u001b[32mffmpegd\u001b[0m] - websocket server for ffmpeg-commander.\n"
	usage       = `
Usage:
	ffmpegd [port]
	ffmpegd version -- This version.
	`
	progressInterval = time.Second * 1
)

var (
	clients   = make(map[*websocket.Conn]bool)
	broadcast = make(chan Message)
	upgrader  = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}
	progressCh chan struct{}
)

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
	Err     string  `json:"err"`
}

func main() {
	// CLI Banner.
	printBanner()

	// Check if FFmpeg/FFprobe are available.
	err := verifyFFmpeg()
	if err != nil {
		fmt.Println("\u001b[31m" + err.Error() + "\u001b[0m")
		fmt.Println("\u001b[31mPlease ensure FFmpeg and FFprobe are installed and available on $PATH.\u001b[0m")
		return
	}

	// HTTP/WS Server.
	startServer()
}

func printBanner() {
	fmt.Println(logo)
	fmt.Println(description)
}

func startServer() {
	http.HandleFunc("/ws", handleConnections)

	// Handles incoming WS messages from client.
	go handleMessages()

	fmt.Println("  Server started on port \u001b[33m:8080\u001b[0m.")
	fmt.Println("  - Go to \u001b[33mhttps://alfg.github.io/ffmpeg-commander\u001b[0m to connect!")
	fmt.Println("  - \u001b[33mffmpegd\u001b[0m must be enabled in ffmpeg-commander options")
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

func verifyFFmpeg() error {
	ffmpeg := &FFmpeg{}
	version, err := ffmpeg.Version()
	if err != nil {
		return err
	}
	fmt.Println("  Checking FFmpeg version....\u001b[32m" + version + "\u001b[0m")

	ffprobe := &FFProbe{}
	version, err = ffprobe.Version()
	if err != nil {
		return err
	}
	fmt.Println("  Checking FFprobe version...\u001b[32m" + version + "\u001b[0m\n")
	return nil
}

func runEncode(input, output, payload string) {
	probe := FFProbe{}
	probeData, err := probe.Run(input)
	if err != nil {
		sendError(err)
		return
	}

	ffmpeg := &FFmpeg{}
	go trackEncodeProgress(probeData, ffmpeg)
	err = ffmpeg.Run(input, output, payload)

	// If we get an error back from ffmpeg, send an error ws message to clients.
	if err != nil {
		close(progressCh)
		sendError(err)
		return
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

func sendError(err error) {
	for client := range clients {
		p := &Status{
			Err: err.Error(),
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
	ticker := time.NewTicker(progressInterval)

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

				fmt.Printf("\rEncoding... %d / %d (%0.2f%%) %s @ %0.2f fps", currentFrame, totalFrames, pct, speed, fps)

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
