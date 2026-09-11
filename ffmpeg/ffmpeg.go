package ffmpeg

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

const ffmpegCmd = "ffmpeg"

// ErrCancelled is returned by Run when the encode was stopped with Cancel.
var ErrCancelled = errors.New("cancelled")

// FFmpeg runs one encode at a time and tracks its progress.
type FFmpeg struct {
	mu        sync.Mutex
	cmd       *exec.Cmd
	cancelled bool
	progress  Progress
}

// Progress is a snapshot of a running encode, as reported by ffmpeg -progress.
type Progress struct {
	Pass      int // Current pass, starting at 1.
	Passes    int
	Frame     int
	FPS       float64
	OutTimeUS int64 // Position in the output, in microseconds.
	Speed     string
}

// Run builds and runs an encode from an ffmpeg-commander payload.
func (f *FFmpeg) Run(input, output, payload string) error {
	e, err := NewEncode(input, output, payload)
	if err != nil {
		return err
	}
	defer e.Close()
	return f.RunEncode(e)
}

// RunEncode runs each pass of an encode in turn.
func (f *FFmpeg) RunEncode(e *Encode) error {
	passes := e.Passes()
	for i, args := range passes {
		f.mu.Lock()
		if f.cancelled {
			f.mu.Unlock()
			return ErrCancelled
		}
		f.progress = Progress{Pass: i + 1, Passes: len(passes)}
		f.mu.Unlock()

		if err := f.run(args); err != nil {
			return err
		}
	}
	return nil
}

func (f *FFmpeg) run(args []string) error {
	cmd := exec.Command(ffmpegCmd, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}

	// Capture stderr (if any).
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	f.mu.Lock()
	if f.cancelled {
		f.mu.Unlock()
		return ErrCancelled
	}
	if err := cmd.Start(); err != nil {
		f.mu.Unlock()
		return err
	}
	f.cmd = cmd
	f.mu.Unlock()

	f.updateProgress(stdout)

	err = cmd.Wait()

	f.mu.Lock()
	defer f.mu.Unlock()
	f.cmd = nil
	if f.cancelled {
		return ErrCancelled
	}
	if err != nil {
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return errors.New(msg)
		}
		return err
	}
	return nil
}

// Cancel stops the encode. Safe to call at any time, from any goroutine.
func (f *FFmpeg) Cancel() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.cancelled = true
	if f.cmd != nil && f.cmd.Process != nil {
		f.cmd.Process.Kill()
	}
}

// Progress returns the latest progress of the encode.
func (f *FFmpeg) Progress() Progress {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.progress
}

// Version gets the ffmpeg version.
func (f *FFmpeg) Version() (string, error) {
	out, err := exec.Command(ffmpegCmd, "-version").Output()
	if err != nil {
		return "", errors.New("ffmpeg not available on $PATH")
	}
	return parseVersion(out), nil
}

var versionRe = regexp.MustCompile(`version (\S+)`)

// parseVersion reads the version from the first line of -version output, such
// as "7.1.1", "7.1" or a git build like "N-118896-g1bb7c2b".
func parseVersion(out []byte) string {
	line, _, _ := bytes.Cut(out, []byte("\n"))
	if m := versionRe.FindSubmatch(line); m != nil {
		return string(m[1])
	}
	return "unknown"
}

// updateProgress reads the key=value lines written by -progress until ffmpeg
// closes stdout.
func (f *FFmpeg) updateProgress(stdout io.Reader) {
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		k, v, ok := strings.Cut(strings.TrimSpace(scanner.Text()), "=")
		if !ok {
			continue
		}
		f.setProgress(k, v)
	}
	// Drain anything left so ffmpeg never blocks writing to a full pipe.
	io.Copy(io.Discard, stdout)
}

func (f *FFmpeg) setProgress(k, v string) {
	f.mu.Lock()
	defer f.mu.Unlock()

	switch k {
	case "frame":
		if n, err := strconv.Atoi(v); err == nil {
			f.progress.Frame = n
		}
	case "fps":
		if n, err := strconv.ParseFloat(v, 64); err == nil {
			f.progress.FPS = n
		}
	case "out_time_us", "out_time_ms": // Both are microseconds.
		if n, err := strconv.ParseInt(v, 10, 64); err == nil && n >= 0 {
			f.progress.OutTimeUS = n
		}
	case "speed":
		f.progress.Speed = v
	}
}
