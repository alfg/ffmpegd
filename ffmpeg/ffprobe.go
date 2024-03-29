package ffmpeg

import (
	"encoding/json"
	"errors"
	"os/exec"
	"regexp"
	"strings"
)

const ffprobeCmd = "ffprobe"

// FFProbe struct.
type FFProbe struct{}

// Run runs an FFProbe command.
func (f FFProbe) Run(input string) (*FFProbeResponse, error) {
	args := []string{
		"-i", input,
		"-show_streams",
		"-print_format", "json",
		"-v", "error",
	}

	// Execute command.
	cmd := exec.Command(ffprobeCmd, args...)
	stdout, err := cmd.CombinedOutput()
	if err != nil {
		// Cleanup ffprobe error output.
		replacer := strings.NewReplacer("{", "", "}", "")
		stdout := strings.TrimSpace(replacer.Replace(string(stdout)))
		return nil, errors.New(stdout)
	}

	dat := &FFProbeResponse{}
	if err := json.Unmarshal([]byte(stdout), &dat); err != nil {
		panic(err)
	}
	return dat, nil
}

// Version gets the ffprobe version.
func (f *FFProbe) Version() (string, error) {
	out, err := exec.Command(ffprobeCmd, "-version").Output()
	if err != nil {
		return "", errors.New("ffprobe not available on $PATH")
	}
	str := strings.Split(string(out), "\n")
	r, _ := regexp.Compile(`(\d+(\.\d+){2})`)
	version := r.FindString(str[0])
	return version, nil
}

// FFProbeResponse defines the response from ffprobe.
type FFProbeResponse struct {
	Streams []stream `json:"streams"`
}

type stream struct {
	Index              int         `json:"index"`
	CodecName          string      `json:"codec_name"`
	CodecLongName      string      `json:"codec_long_name"`
	Profile            string      `json:"profile"`
	CodecType          string      `json:"codec_type"`
	CodecTimeBase      string      `json:"codec_time_base"`
	CodecTagString     string      `json:"codec_tag_string"`
	CodecTag           string      `json:"codec_tag"`
	Width              int         `json:"width"`
	Height             int         `json:"height"`
	CodedWidth         int         `json:"coded_width"`
	CodedHeight        int         `json:"coded_height"`
	HasBFrames         int         `json:"has_b_frames"`
	SampleAspectRatio  string      `json:"sample_aspect_ratio"`
	DisplayAspectRatio string      `json:"display_aspect_ratio"`
	PixFmt             string      `json:"pix_fmt"`
	Level              int         `json:"level"`
	ChromaLocation     string      `json:"chroma_location"`
	Refs               int         `json:"refs"`
	IsAVC              string      `json:"is_avc"`
	NalLengthSize      string      `json:"nal_length_size"`
	RFrameRate         string      `json:"r_frame_rate"`
	AvgFrameRate       string      `json:"avg_frame_rate"`
	TimeBase           string      `json:"time_base"`
	StartPts           int         `json:"start_pts"`
	StartTime          string      `json:"start_time"`
	DurationTS         int         `json:"duration_ts"`
	Duration           string      `json:"duration"`
	BitRate            string      `json:"bit_rate"`
	BitsPerRawSample   string      `json:"bits_per_raw_sample"`
	NbFrames           string      `json:"nb_frames"`
	Disposition        disposition `json:"disposition"`
	Tags               tags        `json:"tags"`
}

type disposition struct {
	Default         int `json:"default"`
	Dub             int `json:"dub"`
	Original        int `json:"original"`
	Comment         int `json:"comment"`
	Lyrics          int `json:"lyrics"`
	Karoake         int `json:"karaoke"`
	Forced          int `json:"forced"`
	HearingImpaired int `json:"hearing_impaired"`
	VisualImpaired  int `json:"visual_empaired"`
	CleanEffects    int `json:"clean_effects"`
	AttachedPic     int `json:"attached_pic"`
	TimedThumbnails int `json:"timed_thumbnails"`
}

type tags struct {
	Language    string `json:"language"`
	HandlerName string `json:"handler_name"`
}
