package cmd

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/alfg/ffmpegd/ffmpeg"
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
                                                      v0.0.7
	`
	version     = "ffmpegd version 0.0.7"
	description = "[\u001b[32mffmpegd\u001b[0m] - websocket server for \u001b[33mffmpeg-commander\u001b[0m.\n"
	usage       = `
Usage:
  ffmpegd            Run server.
  ffmpegd [port]     Run server on port.
  ffmpegd version    Print version.
  ffmpegd help       This help text.
	`
	progressInterval = time.Second * 1
)

var (
	port           = "8080"
	allowedOrigins = []string{
		"http://localhost:" + port,
		"https://alfg.github.io",
	}
	clients   = make(map[*websocket.Conn]bool)
	broadcast = make(chan Message)
	upgrader  = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			for _, origin := range allowedOrigins {
				if r.Header.Get("Origin") == origin {
					return true
				}
			}
			return false
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
	Err     string  `json:"err,omitempty"`
}

// FilesResponse http response for files endpoint.
type FilesResponse struct {
	Cwd     string   `json:"cwd"`
	Folders []string `json:"folders"`
	Files   []file   `json:"files"`
}

type file struct {
	Name string `json:"name"`
	Size int64  `json:"size"`
}

func Run() {
	parseArgs()

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

func parseArgs() {
	args := os.Args

	// Use defaults if no args are set.
	if len(args) == 1 {
		return
	}

	// Print version, help or set port.
	if args[1] == "version" || args[1] == "-v" {
		fmt.Println(version)
		os.Exit(1)
	} else if args[1] == "help" || args[1] == "-h" {
		fmt.Println(usage)
		os.Exit(1)
	} else if _, err := strconv.Atoi(args[1]); err == nil {
		port = args[1]
	}

}

func printBanner() {
	fmt.Println(logo)
	fmt.Println(description)
}

func startServer() {
	http.HandleFunc("/ws", handleConnections)
	http.HandleFunc("/files", handleFiles)
	http.Handle("/", http.FileServer(http.Dir("./")))

	// Handles incoming WS messages from client.
	go handleMessages()

	fmt.Println("  Server started on port \u001b[33m:" + port + "\u001b[0m.")
	fmt.Println("  - Go to \u001b[33mhttps://alfg.github.io/ffmpeg-commander\u001b[0m to connect!")
	fmt.Println("  - \u001b[33mffmpegd\u001b[0m must be enabled in ffmpeg-commander options.")
	fmt.Println("")
	fmt.Printf("Waiting for connection...")
	err := http.ListenAndServe(":"+port, nil)
	if err != nil {
		fmt.Println("ListenAndServe: ", err)
	}
}

func handleConnections(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Printf("\rWaiting for connection...\u001b[31m websocket connection failed!\u001b[0m")
		return
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

func handleFiles(w http.ResponseWriter, r *http.Request) {
	prefix := r.URL.Query().Get("prefix")
	if prefix == "" {
		prefix = "."
	}
	prefix = strings.TrimSuffix(prefix, "/")

	wd, _ := os.Getwd()
	resp := &FilesResponse{
		Cwd:     wd,
		Folders: []string{},
		Files:   []file{},
	}

	files, _ := ioutil.ReadDir(prefix)
	for _, f := range files {
		if f.IsDir() {
			if prefix == "." {
				resp.Folders = append(resp.Folders, f.Name()+"/")
			} else {
				resp.Folders = append(resp.Folders, prefix+"/"+f.Name()+"/")
			}
		} else {
			var obj file
			if prefix == "./" {
				obj.Name = prefix + f.Name()
			} else {
				obj.Name = prefix + "/" + f.Name()
			}
			obj.Size = f.Size()
			resp.Files = append(resp.Files, obj)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	cors(&w, r)
	json.NewEncoder(w).Encode(resp)
}

func cors(w *http.ResponseWriter, r *http.Request) {
	for _, origin := range allowedOrigins {
		if r.Header.Get("Origin") == origin {
			(*w).Header().Set("Access-Control-Allow-Origin", origin)
		}
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
	f := &ffmpeg.FFmpeg{}
	version, err := f.Version()
	if err != nil {
		return err
	}
	fmt.Println("  Checking FFmpeg version....\u001b[32m" + version + "\u001b[0m")

	probe := &ffmpeg.FFProbe{}
	version, err = probe.Version()
	if err != nil {
		return err
	}
	fmt.Println("  Checking FFprobe version...\u001b[32m" + version + "\u001b[0m\n")
	return nil
}

func runEncode(input, output, payload string) {
	probe := ffmpeg.FFProbe{}
	probeData, err := probe.Run(input)
	if err != nil {
		sendError(err)
		return
	}

	ffmpeg := &ffmpeg.FFmpeg{}
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

func trackEncodeProgress(p *ffmpeg.FFProbeResponse, f *ffmpeg.FFmpeg) {
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
