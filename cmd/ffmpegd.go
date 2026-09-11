package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
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
                                                      v0.2.1
	`
	version     = "ffmpegd version 0.2.1"
	description = "[\u001b[32mffmpegd\u001b[0m] - websocket server for \u001b[33mffmpeg-commander\u001b[0m.\n"
	usage       = `
Usage:
  ffmpegd [--host address] [port]   Run server on localhost:8080 by default.
  ffmpegd version                   Print version.
  ffmpegd help                      This help text.

Options:
  --host address   Address to listen on. Use 0.0.0.0 to let other machines on
                   your network connect. Can also be set with $FFMPEGD_HOST.
	`
	progressInterval = time.Second * 1
	writeTimeout     = time.Second * 10
	maxQueuedJobs    = 100
)

var (
	port = "8080"
	host = "localhost"

	// Sites allowed to connect, besides the daemon's own localhost origin.
	remoteOrigins = []string{
		"https://alfg.github.io",
		"https://alfg.dev",
		"https://ffmpeg-commander.com",
		"https://www.ffmpeg-commander.com",
	}
	allowedOrigins []string

	clients  = &hub{conns: make(map[*websocket.Conn]bool)}
	jobs     = make(chan Message, maxQueuedJobs)
	upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return isAllowedOrigin(r.Header.Get("Origin"))
		},
	}

	// The running encode, so a cancel message can stop it.
	currentMu sync.Mutex
	current   *ffmpeg.FFmpeg
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
	Percent   float64 `json:"percent"`
	Speed     string  `json:"speed"`
	FPS       float64 `json:"fps"`
	Err       string  `json:"err,omitempty"`
	Cancelled bool    `json:"cancelled,omitempty"`
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

// hub is the set of connected clients. gorilla/websocket allows one writer at
// a time per connection, so writes go through the hub's lock too.
type hub struct {
	mu    sync.Mutex
	conns map[*websocket.Conn]bool
}

func (h *hub) add(c *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.conns[c] = true
}

func (h *hub) remove(c *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.conns, c)
	c.Close()
}

// send writes a message to one client, dropping it if the write fails.
func (h *hub) send(c *websocket.Conn, v any) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.write(c, v)
}

// broadcast writes a message to every client.
func (h *hub) broadcast(v any) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for c := range h.conns {
		h.write(c, v)
	}
}

func (h *hub) write(c *websocket.Conn, v any) {
	c.SetWriteDeadline(time.Now().Add(writeTimeout))
	if err := c.WriteJSON(v); err != nil {
		delete(h.conns, c)
		c.Close()
	}
}

func Run() {
	if err := parseArgs(os.Args[1:]); err != nil {
		fmt.Println("\u001b[31m" + err.Error() + "\u001b[0m")
		fmt.Println(usage)
		os.Exit(2)
	}
	allowedOrigins = append([]string{
		"http://localhost:" + port,
		"http://127.0.0.1:" + port,
	}, remoteOrigins...)

	// CLI Banner.
	printBanner()

	// Check if FFmpeg/FFprobe are available.
	err := verifyFFmpeg()
	if err != nil {
		fmt.Println("\u001b[31m" + err.Error() + "\u001b[0m")
		fmt.Println("\u001b[31mPlease ensure FFmpeg and FFprobe are installed and available on $PATH.\u001b[0m")
		os.Exit(1)
	}

	// HTTP/WS Server.
	startServer()
}

func parseArgs(args []string) error {
	if h := os.Getenv("FFMPEGD_HOST"); h != "" {
		host = h
	}

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "version" || arg == "-v" || arg == "--version":
			fmt.Println(version)
			os.Exit(0)
		case arg == "help" || arg == "-h" || arg == "--help":
			fmt.Println(usage)
			os.Exit(0)
		case arg == "--host" || arg == "-host":
			if i+1 == len(args) {
				return errors.New("--host needs an address, such as 0.0.0.0")
			}
			i++
			host = args[i]
		case strings.HasPrefix(arg, "--host="):
			host = strings.TrimPrefix(arg, "--host=")
		default:
			if n, err := strconv.Atoi(arg); err != nil || n < 1 || n > 65535 {
				return fmt.Errorf("unknown argument %q", arg)
			}
			port = arg
		}
	}
	return nil
}

func printBanner() {
	fmt.Println(logo)
	fmt.Print(description + "\n")
}

// listen opens the server's listeners. "localhost" listens on both loopback
// addresses, since browsers may resolve it to either.
func listen() ([]net.Listener, error) {
	if host != "localhost" {
		l, err := net.Listen("tcp", net.JoinHostPort(host, port))
		if err != nil {
			return nil, err
		}
		return []net.Listener{l}, nil
	}

	l, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", port))
	if err != nil {
		return nil, err
	}
	listeners := []net.Listener{l}
	// IPv6 may be unavailable; IPv4 alone is enough.
	if l6, err := net.Listen("tcp", net.JoinHostPort("::1", port)); err == nil {
		listeners = append(listeners, l6)
	}
	return listeners, nil
}

func isLoopback(h string) bool {
	if h == "localhost" {
		return true
	}
	ip := net.ParseIP(h)
	return ip != nil && ip.IsLoopback()
}

func startServer() {
	http.HandleFunc("/ws", handleConnections)
	http.HandleFunc("/files", handleFiles)
	http.Handle("/", http.FileServer(http.Dir("./")))

	listeners, err := listen()
	if err != nil {
		fmt.Println("\u001b[31mCould not start server: " + err.Error() + "\u001b[0m")
		if strings.Contains(err.Error(), "address already in use") {
			fmt.Println("Is ffmpegd already running? To use another port, run: ffmpegd 8081")
		}
		os.Exit(1)
	}

	// Handles queued encodes, one at a time.
	go handleMessages()

	fmt.Println("  Server started on \u001b[33mhttp://" + net.JoinHostPort(host, port) + "\u001b[0m.")
	if !isLoopback(host) {
		fmt.Println("  \u001b[31mListening on " + host + ", so other machines may be able to connect.\u001b[0m")
	}
	fmt.Println("  - Go to \u001b[33mhttps://ffmpeg-commander.com\u001b[0m to connect!")
	fmt.Println("  - \u001b[33mffmpegd\u001b[0m must be enabled in ffmpeg-commander options.")
	fmt.Println("")
	fmt.Printf("Waiting for connection...")

	errs := make(chan error, len(listeners))
	for _, l := range listeners {
		go func(l net.Listener) { errs <- http.Serve(l, nil) }(l)
	}
	fmt.Println("\n\u001b[31mServer stopped: " + (<-errs).Error() + "\u001b[0m")
	os.Exit(1)
}

func isAllowedOrigin(origin string) bool {
	for _, o := range allowedOrigins {
		if origin == o {
			return true
		}
	}
	return false
}

func handleConnections(w http.ResponseWriter, r *http.Request) {
	if origin := r.Header.Get("Origin"); !isAllowedOrigin(origin) {
		fmt.Printf("\rRejected connection from %q: not an allowed origin.%s\n", origin, clearLine)
		http.Error(w, "origin not allowed", http.StatusForbidden)
		return
	}
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Printf("\rWaiting for connection...\u001b[31m websocket connection failed!\u001b[0m")
		return
	}
	clients.add(ws)
	defer clients.remove(ws)

	fmt.Printf("\rWaiting for connection......\u001b[32mconnected!\u001b[0m")
	for {
		_, data, err := ws.ReadMessage()
		if err != nil {
			fmt.Printf("\rWaiting for connection...\u001b[31mdisconnected!\u001b[0m")
			return
		}

		var msg Message
		if err := json.Unmarshal(data, &msg); err != nil {
			clients.send(ws, Status{Err: "invalid message: " + err.Error()})
			continue
		}

		switch msg.Type {
		case "encode":
			select {
			case jobs <- msg:
			default:
				clients.send(ws, Status{Err: "too many encodes queued, try again later"})
			}
		case "cancel":
			cancelEncode()
		default:
			clients.send(ws, Status{Err: fmt.Sprintf("unknown message type %q", msg.Type)})
		}
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

	files, _ := os.ReadDir(cleanPath(prefix))
	for _, f := range files {
		if f.IsDir() {
			if prefix == "." {
				resp.Folders = append(resp.Folders, f.Name()+"/")
			} else {
				resp.Folders = append(resp.Folders, prefix+"/"+f.Name()+"/")
			}
		} else {
			info, err := f.Info()
			if err != nil {
				continue // Removed since the directory was read.
			}
			obj := file{Name: prefix + "/" + f.Name(), Size: info.Size()}
			if prefix == "." {
				obj.Name = f.Name()
			}
			resp.Files = append(resp.Files, obj)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	cors(w, r)
	json.NewEncoder(w).Encode(resp)
}

func cleanPath(path string) string {
	// Clean path to make safe for use with filepath.Join.
	if path == "" {
		return ""
	}
	path = filepath.Clean(path)
	if !filepath.IsAbs(path) {
		path = filepath.Clean(string(os.PathSeparator) + path)
		path, _ = filepath.Rel(string(os.PathSeparator), path)
	}
	return filepath.Clean(path)
}

func cors(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Vary", "Origin")
	if origin := r.Header.Get("Origin"); isAllowedOrigin(origin) {
		w.Header().Set("Access-Control-Allow-Origin", origin)
	}
}

func handleMessages() {
	for msg := range jobs {
		runEncode(msg)
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

func cancelEncode() {
	currentMu.Lock()
	defer currentMu.Unlock()
	if current != nil {
		current.Cancel()
	}
}

func runEncode(msg Message) {
	// Registered before anything slow, so a cancel that arrives while the
	// input is still being probed is not lost.
	f := &ffmpeg.FFmpeg{}
	currentMu.Lock()
	current = f
	currentMu.Unlock()
	defer func() {
		currentMu.Lock()
		current = nil
		currentMu.Unlock()
	}()

	fmt.Printf("\rEncoding %s -> %s%s\n", msg.Input, msg.Output, clearLine)

	e, err := ffmpeg.NewEncode(msg.Input, msg.Output, msg.Payload)
	if err != nil {
		sendError(err)
		return
	}
	defer e.Close()

	probe, err := ffmpeg.FFProbe{}.Run(msg.Input)
	if err != nil {
		sendError(err)
		return
	}

	done := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		trackEncodeProgress(f, e.OutputDuration(probe.Duration()), totalFrames(probe), done)
	}()
	err = f.RunEncode(e)
	// Stop progress updates before the final status so none can follow it.
	close(done)
	wg.Wait()

	switch {
	case errors.Is(err, ffmpeg.ErrCancelled):
		fmt.Printf("\rEncode cancelled.%s\n", clearLine)
		clients.broadcast(Status{Cancelled: true})
	case err != nil:
		sendError(err)
	default:
		fmt.Printf("\rEncode finished: %s%s\n", msg.Output, clearLine)
		clients.broadcast(Status{Percent: 100})
	}
	fmt.Printf("Waiting for next job...")
}

// clearLine erases what's left of a progress line after \r.
const clearLine = "\u001b[K"

func sendError(err error) {
	fmt.Printf("\r\u001b[31mEncode failed:\u001b[0m %s%s\n", err, clearLine)
	clients.broadcast(Status{Err: err.Error()})
}

// totalFrames returns the frame count of the first video stream, or 0.
func totalFrames(p *ffmpeg.FFProbeResponse) int {
	for _, s := range p.Streams {
		if s.CodecType == "video" {
			n, _ := strconv.Atoi(s.NbFrames)
			return n
		}
	}
	return 0
}

// trackEncodeProgress reports progress to clients until done is closed.
// Progress is measured against the output duration when it is known, and the
// frame count otherwise. Each pass of a two-pass encode is half the total.
func trackEncodeProgress(f *ffmpeg.FFmpeg, duration float64, frames int, done <-chan struct{}) {
	ticker := time.NewTicker(progressInterval)
	defer ticker.Stop()

	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			p := f.Progress()
			if p.Passes == 0 {
				continue
			}

			frac := -1.0
			switch {
			case duration > 0:
				frac = float64(p.OutTimeUS) / 1e6 / duration
			case frames > 0:
				frac = float64(p.Frame) / float64(frames)
			}

			var pct float64
			if frac >= 0 {
				pct = (float64(p.Pass-1) + math.Min(frac, 1)) / float64(p.Passes) * 100
				// 100 tells the client the job is done, so hold just short of
				// it until ffmpeg exits.
				pct = math.Min(math.Round(pct*100)/100, 99.99)
			}

			pass := ""
			if p.Passes > 1 {
				pass = fmt.Sprintf(" pass %d/%d", p.Pass, p.Passes)
			}
			if frac >= 0 {
				fmt.Printf("\rEncoding...%s %0.2f%% %s @ %0.2f fps%s", pass, pct, p.Speed, p.FPS, clearLine)
			} else {
				fmt.Printf("\rEncoding...%s %0.1fs done %s @ %0.2f fps%s", pass, float64(p.OutTimeUS)/1e6, p.Speed, p.FPS, clearLine)
			}

			clients.broadcast(Status{Percent: pct, Speed: p.Speed, FPS: p.FPS})
		}
	}
}
