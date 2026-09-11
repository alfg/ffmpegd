package ffmpeg

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Encode is an encode job built from an ffmpeg-commander payload: the ffmpeg
// arguments for each pass, and what is needed to report progress.
type Encode struct {
	passes  [][]string
	options *ffmpegOptions
	logDir  string // Holds the two-pass log files, removed by Close.
}

// NewEncode builds the ffmpeg arguments for a payload.
//
// The flags mirror the command ffmpeg-commander shows the user:
// https://github.com/alfg/ffmpeg-commander/blob/master/src/lib/ffmpeg.ts
func NewEncode(input, output, payload string) (*Encode, error) {
	options := &ffmpegOptions{}
	if err := json.Unmarshal([]byte(payload), options); err != nil {
		return nil, fmt.Errorf("invalid encode options: %w", err)
	}

	global := []string{
		"-hide_banner",
		"-nostdin",
		"-nostats",
		"-loglevel", "error", // Set loglevel to fail job on errors.
		"-progress", "pipe:1",
		"-y",
		"-i", input,
	}
	e := &Encode{options: options}

	// If raw options provided, add the list of raw options from ffmpeg presets.
	if len(options.Raw) > 0 {
		args := append([]string{}, global...)
		for _, v := range options.Raw {
			args = append(args, strings.Fields(v)...)
		}
		e.passes = [][]string{append(args, output)}
		return e, nil
	}

	args := append(global, transformOptions(options)...)

	if string(options.Video.Pass) == "2" {
		dir, err := os.MkdirTemp("", "ffmpegd-2pass-")
		if err != nil {
			return nil, err
		}
		e.logDir = dir
		first, second := set2Pass(args, options, dir)
		e.passes = [][]string{first, append(second, output)}
		return e, nil
	}

	e.passes = [][]string{append(args, output)}
	return e, nil
}

// Passes returns the ffmpeg arguments for each pass, in order.
func (e *Encode) Passes() [][]string {
	return e.passes
}

// OutputDuration estimates the length of the output in seconds, given the
// input's length, by applying the clip range and speed options. Returns 0 when
// the input length is unknown.
func (e *Encode) OutputDuration(input float64) float64 {
	if input <= 0 {
		return 0
	}
	d := input

	f := e.options.Format
	if f.Clip {
		start := parseTime(string(f.StartTime))
		if stop := parseTime(string(f.StopTime)); stop > 0 && stop < d {
			d = stop
		}
		d -= start
	}

	// setpts=0.5*PTS halves the output length.
	if factor, ok := strings.CutSuffix(string(e.options.Video.Speed), "*PTS"); ok {
		if v, err := strconv.ParseFloat(factor, 64); err == nil && v > 0 {
			d *= v
		}
	}

	if d < 0 {
		return 0
	}
	return d
}

// Close removes any files the encode left behind.
func (e *Encode) Close() {
	if e.logDir != "" {
		os.RemoveAll(e.logDir)
	}
}

// ffmpegOptions is the JSON payload sent by ffmpeg-commander.
type ffmpegOptions struct {
	Format formatOptions `json:"format"`
	Video  videoOptions  `json:"video"`
	Audio  audioOptions  `json:"audio"`
	Filter filterOptions `json:"filter"`

	Raw []string `json:"raw"` // Raw flag options.
}

type formatOptions struct {
	Container optString `json:"container"`
	Clip      optBool   `json:"clip"`
	StartTime optString `json:"startTime"`
	StopTime  optString `json:"stopTime"`
}

type videoOptions struct {
	Codec        optString `json:"codec"`
	Preset       optString `json:"preset"`
	Pass         optString `json:"pass"`
	Crf          optString `json:"crf"`
	Bitrate      optString `json:"bitrate"`
	MinRate      optString `json:"minrate"`
	MaxRate      optString `json:"maxrate"`
	BufSize      optString `json:"bufsize"`
	GopSize      optString `json:"gopsize"`
	PixelFormat  optString `json:"pixel_format"`
	FrameRate    optString `json:"frame_rate"`
	Speed        optString `json:"speed"`
	Tune         optString `json:"tune"`
	Profile      optString `json:"profile"`
	Level        optString `json:"level"`
	FastStart    optBool   `json:"faststart"`
	Size         optString `json:"size"`
	Width        optString `json:"width"`
	Height       optString `json:"height"`
	Fit          optBool   `json:"fit"`
	Format       optString `json:"format"`
	Aspect       optString `json:"aspect"`
	Scaling      optString `json:"scaling"`
	CodecOptions optString `json:"codec_options"`
}

type audioOptions struct {
	Codec   optString `json:"codec"`
	Channel optString `json:"channel"`
	Quality optString `json:"quality"`
	Bitrate optString `json:"bitrate"` // Used when quality is "custom".
	Volume  optString `json:"volume"`

	// ffmpeg-commander sends sampleRate; sample_rate is still accepted from
	// older clients.
	SampleRate       optString `json:"sampleRate"`
	SampleRateLegacy optString `json:"sample_rate"`
}

type filterOptions struct {
	Deband      optBool   `json:"deband"`
	Deshake     optBool   `json:"deshake"`
	Deflicker   optBool   `json:"deflicker"`
	Dejudder    optBool   `json:"dejudder"`
	Denoise     optString `json:"denoise"`
	Deinterlace optString `json:"deinterlace"`
	Brightness  optString `json:"brightness"`
	Contrast    optString `json:"contrast"`
	Saturation  optString `json:"saturation"`
	Gamma       optString `json:"gamma"`
	Acontrast   optString `json:"acontrast"`
	Adelay      optString `json:"adelay"`
}

// optString is an option value that may arrive as a JSON string, number or
// null. ffmpeg-commander sends 23 or "23" for the same field depending on how
// the value was set, so both must decode.
type optString string

func (s *optString) UnmarshalJSON(b []byte) error {
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	switch v := v.(type) {
	case nil:
		*s = ""
	case string:
		*s = optString(v)
	case float64:
		*s = optString(strconv.FormatFloat(v, 'f', -1, 64))
	case bool:
		*s = optString(strconv.FormatBool(v))
	default:
		return fmt.Errorf("expected a string or number, got %s", b)
	}
	return nil
}

// set reports whether the option has a value other than the "none" and "auto"
// placeholders the form uses for "leave it to ffmpeg".
func (s optString) set() bool {
	return s != "" && s != "none" && s != "auto"
}

// optBool is a boolean option that may also arrive as "true"/"false" or null.
type optBool bool

func (b *optBool) UnmarshalJSON(data []byte) error {
	var v any
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	switch v := v.(type) {
	case nil:
		*b = false
	case bool:
		*b = optBool(v)
	case string:
		*b = v == "true"
	default:
		return fmt.Errorf("expected true or false, got %s", data)
	}
	return nil
}

// number parses an option as a float, reporting false if it is not a number.
func number(s optString) (float64, bool) {
	v, err := strconv.ParseFloat(strings.TrimSpace(string(s)), 64)
	return v, err == nil
}

// formatNumber prints a float the way JavaScript would: 0.5, not 0.500000.
func formatNumber(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}

// codecArgs splits a codec value into arguments. Some map to more than one:
// ffmpeg-commander's mpeg4 is "mpeg4 -vtag xvid".
func codecArgs(flag string, codec optString) []string {
	fields := strings.Fields(string(codec))
	if len(fields) == 0 {
		return nil
	}
	return append([]string{flag, fields[0]}, fields[1:]...)
}

// transformOptions converts the ffmpegOptions{} struct into a slice of ffmpeg
// options to be passed to exec.Command arguments.
func transformOptions(opt *ffmpegOptions) []string {
	args := []string{}

	// Set format flags if clip options are set.
	if opt.Format.Clip {
		args = append(args, setFormatFlags(opt.Format)...)
	}

	// Video flags and filters. Filters are skipped when there is no video track.
	args = append(args, setVideoFlags(opt.Video)...)
	if opt.Video.Codec != "none" {
		if vf := setVideoFilters(opt.Video, opt.Filter); vf != "" {
			args = append(args, "-vf", vf)
		}
	}

	// Audio flags and filters. Filters are skipped when there is no audio track.
	args = append(args, setAudioFlags(opt.Audio)...)
	if !noAudio(opt.Audio) {
		if af := setAudioFilters(opt.Audio, opt.Filter); af != "" {
			args = append(args, "-af", af)
		}
	}

	return args
}

func setFormatFlags(opt formatOptions) []string {
	args := []string{}
	if opt.StartTime != "" {
		args = append(args, "-ss", string(opt.StartTime))
	}
	if opt.StopTime != "" {
		args = append(args, "-to", string(opt.StopTime))
	}
	return args
}

func setVideoFlags(opt videoOptions) []string {
	// "None" means no video track at all, so -vn replaces every other flag.
	if opt.Codec == "none" {
		return []string{"-vn"}
	}

	args := []string{}
	if opt.Codec.set() {
		args = append(args, codecArgs("-c:v", opt.Codec)...)
	}

	// Flags that take the option value as-is, in the order ffmpeg-commander
	// emits them. 0 is a real value: -b:v 0 puts VP9 and AV1 in constant
	// quality mode, and VP9 has a profile 0.
	flags := []struct {
		flag  string
		value optString
	}{
		{"-preset", opt.Preset},
		{"-b:v", opt.Bitrate},
		{"-minrate", opt.MinRate},
		{"-maxrate", opt.MaxRate},
		{"-bufsize", opt.BufSize},
		{"-g", opt.GopSize},
		{"-pix_fmt", opt.PixelFormat},
		{"-r", opt.FrameRate},
		{"-tune", opt.Tune},
		{"-profile:v", opt.Profile},
		{"-level", opt.Level},
		{"-aspect", opt.Aspect},
	}
	for _, f := range flags {
		if f.value.set() {
			args = append(args, f.flag, string(f.value))
		}
	}

	if opt.Pass == "crf" && opt.Crf != "" && opt.Crf != "0" {
		args = append(args, "-crf", string(opt.Crf))
	}

	if opt.FastStart {
		args = append(args, "-movflags", "faststart")
	}

	if opt.CodecOptions != "" && (opt.Codec == "libx264" || opt.Codec == "libx265") {
		p := strings.Replace(string(opt.Codec), "lib", "", 1)
		args = append(args, "-"+p+"-params", string(opt.CodecOptions))
	}

	return args
}

func setVideoFilters(vopt videoOptions, opt filterOptions) string {
	args := []string{}

	// Speed.
	if vopt.Speed.set() {
		args = append(args, "setpts="+string(vopt.Speed))
	}

	// Scale.
	scaleFilters := []string{}
	if vopt.Size != "" && vopt.Size != "source" {
		switch {
		case vopt.Size == "custom" && bool(vopt.Fit):
			// Fit inside the box, keeping the source aspect ratio. Rounding to
			// even dimensions keeps encoders like x264 from rejecting it.
			scaleFilters = append(scaleFilters, "scale="+string(vopt.Width)+":"+string(vopt.Height)+
				":force_original_aspect_ratio=decrease:force_divisible_by=2")
		case vopt.Size == "custom":
			scaleFilters = append(scaleFilters, "scale="+string(vopt.Width)+":"+string(vopt.Height))
		// -2 rather than ffmpeg-commander's -1: keep the aspect ratio but round
		// to an even number, which x264 and x265 require. 1280x534 scaled to
		// 1920 wide is 801 high with -1, and the encode fails.
		case vopt.Format == "widescreen":
			scaleFilters = append(scaleFilters, "scale="+string(vopt.Size)+":-2")
		default:
			scaleFilters = append(scaleFilters, "scale=-2:"+string(vopt.Size))
		}
	}

	// flags is an option of scale, so it is only valid alongside a resize.
	if vopt.Scaling.set() && len(scaleFilters) > 0 {
		scaleFilters = append(scaleFilters, "flags="+string(vopt.Scaling))
	}

	if len(scaleFilters) > 0 {
		args = append(args, strings.Join(scaleFilters, ":"))
	}

	// More filters.
	if opt.Deband {
		args = append(args, "deband")
	}
	if opt.Deshake {
		args = append(args, "deshake")
	}
	if opt.Deflicker {
		args = append(args, "deflicker")
	}
	if opt.Dejudder {
		args = append(args, "dejudder")
	}

	if opt.Denoise.set() {
		switch opt.Denoise {
		case "light":
			args = append(args, "removegrain=22")
		case "medium":
			args = append(args, "vaguedenoiser=threshold=3:method=soft:nsteps=5")
		case "heavy":
			args = append(args, "vaguedenoiser=threshold=6:method=soft:nsteps=5")
		default:
			// A general-purpose denoiser at its own default strength. This was
			// removegrain=0, which leaves every plane unchanged.
			args = append(args, "hqdn3d")
		}
	}

	switch opt.Deinterlace {
	case "frame":
		args = append(args, "yadif=0:-1:0")
	case "field":
		args = append(args, "yadif=1:-1:0")
	case "frame_nospatial":
		args = append(args, "yadif=2:-1:0")
	case "field_nospatial":
		args = append(args, "yadif=3:-1:0")
	}

	// EQ filters. ffmpeg-commander sends these already scaled to eq's units,
	// so contrast is neutral at 1 and the rest at 0.
	eq := []string{}
	if v, ok := number(opt.Contrast); ok && v != 1 {
		eq = append(eq, "contrast="+formatNumber(v))
	}
	if v, ok := number(opt.Brightness); ok && v != 0 {
		eq = append(eq, "brightness="+formatNumber(v))
	}
	if v, ok := number(opt.Saturation); ok && v != 0 {
		eq = append(eq, "saturation="+formatNumber(v))
	}
	if v, ok := number(opt.Gamma); ok && v != 0 {
		eq = append(eq, "gamma="+formatNumber(v))
	}
	if len(eq) > 0 {
		args = append(args, "eq="+strings.Join(eq, ":"))
	}

	return strings.Join(args, ",")
}

// noAudio reports whether the output should have no audio track.
func noAudio(opt audioOptions) bool {
	return opt.Codec == "none" || opt.Quality == "mute"
}

func setAudioFlags(opt audioOptions) []string {
	// "None" means no audio track at all, so -an replaces every other flag.
	if noAudio(opt) {
		return []string{"-an"}
	}

	args := []string{}
	if opt.Codec.set() {
		args = append(args, codecArgs("-c:a", opt.Codec)...)
	}
	// ffmpeg's DTS encoder is marked experimental and refuses to run without this.
	if opt.Codec == "dca" {
		args = append(args, "-strict", "-2")
	}

	sampleRate := opt.SampleRate
	if sampleRate == "" {
		sampleRate = opt.SampleRateLegacy
	}
	if sampleRate.set() {
		args = append(args, "-ar", string(sampleRate))
	}

	if opt.Channel != "" && opt.Channel != "source" {
		args = append(args, "-rematrix_maxval", "1.0", "-ac", string(opt.Channel))
	}

	bitrate := opt.Quality
	if bitrate == "custom" {
		bitrate = opt.Bitrate
	}
	if bitrate.set() {
		args = append(args, "-b:a", string(bitrate))
	}

	return args
}

func setAudioFilters(opt audioOptions, filter filterOptions) string {
	args := []string{}

	if v, ok := number(opt.Volume); ok && v != 100 {
		args = append(args, "volume="+formatNumber(v/100))
	}

	// acontrast takes 0-100 itself, the same scale ffmpeg-commander's slider
	// sends; this used to divide by 100, leaving almost no effect. 33, the
	// filter's default, is treated as off.
	if v, ok := number(filter.Acontrast); ok && v != 33 {
		args = append(args, "acontrast="+formatNumber(v))
	}

	// Delay every channel by the same amount, in milliseconds.
	if v, ok := number(filter.Adelay); ok && v > 0 {
		args = append(args, "adelay=delays="+formatNumber(v)+":all=1")
	}

	return strings.Join(args, ",")
}

// set2Pass returns the arguments for both passes of a two-pass encode. The
// second pass still needs the output appended.
//
// Pass 1 only writes the stats log, so its output goes to the null muxer and
// audio is skipped. The log is kept in logDir rather than the working directory
// so concurrent or abandoned encodes cannot trip over each other's files.
func set2Pass(args []string, opt *ffmpegOptions, logDir string) (first, second []string) {
	first = append([]string{}, args...)
	second = append([]string{}, args...)

	if opt.Video.Codec == "libx265" {
		// x265 takes its pass through -x265-params, joining any already there.
		// Params are colon-separated, so a path with a drive letter can't be
		// passed; x265 then falls back to its default log in the working dir.
		p1, p2 := "pass=1", "pass=2"
		if stats := filepath.ToSlash(filepath.Join(logDir, "x265.log")); !strings.Contains(stats, ":") {
			p1 += ":stats=" + stats
			p2 += ":stats=" + stats
		}
		if i := indexOf(first, "-x265-params"); i >= 0 {
			first[i+1] += ":" + p1
			second[i+1] += ":" + p2
		} else {
			first = append(first, "-x265-params", p1)
			second = append(second, "-x265-params", p2)
		}
	} else {
		logFile := filepath.Join(logDir, "ffmpeg2pass")
		first = append(first, "-pass", "1", "-passlogfile", logFile)
		second = append(second, "-pass", "2", "-passlogfile", logFile)
	}

	if indexOf(first, "-an") < 0 {
		first = append(first, "-an")
	}
	first = append(first, "-f", "null", os.DevNull)
	return first, second
}

func indexOf(args []string, s string) int {
	for i, a := range args {
		if a == s {
			return i
		}
	}
	return -1
}

// parseTime parses an ffmpeg time duration, [-][HH:]MM:SS[.m...] or seconds
// with an optional s, ms or us suffix, into seconds. Returns 0 if invalid.
func parseTime(s string) float64 {
	s = strings.TrimSpace(s)
	unit := 1.0
	switch {
	case strings.HasSuffix(s, "ms"):
		s, unit = strings.TrimSuffix(s, "ms"), 1e-3
	case strings.HasSuffix(s, "us"):
		s, unit = strings.TrimSuffix(s, "us"), 1e-6
	case strings.HasSuffix(s, "s"):
		s = strings.TrimSuffix(s, "s")
	}

	var total float64
	for _, part := range strings.Split(s, ":") {
		v, err := strconv.ParseFloat(part, 64)
		if err != nil {
			return 0
		}
		total = total*60 + v
	}
	return total * unit
}
