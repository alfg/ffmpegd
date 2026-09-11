package ffmpeg

import (
	"os"
	"reflect"
	"strings"
	"testing"
)

// defaultPayload is what ffmpeg-commander sends for its default form.
const defaultPayload = `{
	"format": {"container": "mp4", "clip": false, "startTime": null, "stopTime": null},
	"video": {"codec": "libx264", "preset": "none", "pass": "1", "crf": 23, "bitrate": null,
		"minrate": null, "maxrate": null, "bufsize": null, "gopsize": null,
		"pixel_format": "auto", "frame_rate": "auto", "speed": "auto", "tune": "none",
		"profile": "none", "level": "none", "faststart": false, "size": "source",
		"width": "1080", "height": "1920", "format": "widescreen", "aspect": "auto",
		"scaling": "auto", "codec_options": ""},
	"audio": {"codec": "copy", "channel": "source", "quality": "auto", "sampleRate": "auto", "volume": "100"},
	"filter": {"deband": false, "deshake": false, "deflicker": false, "dejudder": false,
		"denoise": "none", "deinterlace": "none", "brightness": "0", "contrast": "1",
		"saturation": "0", "gamma": "0", "acontrast": "33"}
}`

func TestTransformOptions(t *testing.T) {
	tests := []struct {
		name    string
		payload string
		want    string
	}{
		{"commander defaults", defaultPayload, "-c:v libx264 -c:a copy"},
		{"empty payload", `{}`, ""},
		{
			"crf from the slider arrives as a string",
			`{"video": {"codec": "libx264", "pass": "crf", "crf": "28"}}`,
			"-c:v libx264 -crf 28",
		},
		{
			"crf as a number",
			`{"video": {"codec": "libx264", "pass": "crf", "crf": 30}}`,
			"-c:v libx264 -crf 30",
		},
		{
			"crf ignored outside crf mode",
			`{"video": {"codec": "libx264", "pass": "1", "crf": 23}}`,
			"-c:v libx264",
		},
		{
			"audio-only preset",
			`{"video": {"codec": "none"}, "audio": {"codec": "libmp3lame", "quality": "192k", "sampleRate": "44100"}}`,
			"-vn -c:a libmp3lame -ar 44100 -b:a 192k",
		},
		{
			"legacy sample_rate key",
			`{"audio": {"codec": "aac", "sample_rate": "48000"}}`,
			"-c:a aac -ar 48000",
		},
		{
			"no video skips video filters",
			`{"video": {"codec": "none", "size": "720"}, "filter": {"deband": true}}`,
			"-vn",
		},
		{
			"no audio skips audio filters",
			`{"audio": {"codec": "none", "volume": "50"}}`,
			"-an",
		},
		{
			"mute quality drops audio",
			`{"audio": {"codec": "aac", "quality": "mute"}}`,
			"-an",
		},
		{
			"codec with extra flags",
			`{"video": {"codec": "mpeg4 -vtag xvid"}}`,
			"-c:v mpeg4 -vtag xvid",
		},
		{
			"custom audio bit rate",
			`{"audio": {"codec": "aac", "quality": "custom", "bitrate": "144k"}}`,
			"-c:a aac -b:a 144k",
		},
		{
			"blank custom audio bit rate",
			`{"audio": {"codec": "aac", "quality": "custom", "bitrate": ""}}`,
			"-c:a aac",
		},
		{
			"audio channels",
			`{"audio": {"codec": "aac", "channel": "2"}}`,
			"-c:a aac -rematrix_maxval 1.0 -ac 2",
		},
		{
			"frame rate auto with a pixel format",
			`{"video": {"codec": "libx264", "pixel_format": "yuv420p", "frame_rate": "auto"}}`,
			"-c:v libx264 -pix_fmt yuv420p",
		},
		{
			"video flags in commander order",
			`{"video": {"codec": "libx264", "preset": "slow", "bitrate": "2M", "minrate": "1M",
				"maxrate": "3M", "bufsize": "4M", "gopsize": "48", "pixel_format": "yuv420p",
				"frame_rate": "30", "tune": "film", "profile": "high", "level": "4.1",
				"aspect": "16:9", "faststart": true, "codec_options": "keyint=48"}}`,
			"-c:v libx264 -preset slow -b:v 2M -minrate 1M -maxrate 3M -bufsize 4M -g 48 " +
				"-pix_fmt yuv420p -r 30 -tune film -profile:v high -level 4.1 -aspect 16:9 " +
				"-movflags faststart -x264-params keyint=48",
		},
		{
			"zero bit rate for constant quality",
			`{"video": {"codec": "libvpx-vp9", "bitrate": "0", "pass": "crf", "crf": 31}}`,
			"-c:v libvpx-vp9 -b:v 0 -crf 31",
		},
		{
			"vp9 profile 0 is kept",
			`{"video": {"codec": "libvpx-vp9", "profile": "0"}}`,
			"-c:v libvpx-vp9 -profile:v 0",
		},
		{
			"codec params only for x264 and x265",
			`{"video": {"codec": "libvpx-vp9", "codec_options": "foo=1"}}`,
			"-c:v libvpx-vp9",
		},
		{
			"scaling without a resize",
			`{"video": {"codec": "libx264", "scaling": "lanczos"}}`,
			"-c:v libx264",
		},
		{
			"widescreen resize with scaling",
			`{"video": {"size": "1280", "format": "widescreen", "scaling": "lanczos"}}`,
			"-vf scale=1280:-2:flags=lanczos",
		},
		{
			"fullscreen resize",
			`{"video": {"size": "720", "format": "fullscreen"}}`,
			"-vf scale=-2:720",
		},
		{
			"custom size",
			`{"video": {"size": "custom", "width": "640", "height": "360"}}`,
			"-vf scale=640:360",
		},
		{
			"custom size fit inside box",
			`{"video": {"size": "custom", "width": "640", "height": "360", "fit": true}}`,
			"-vf scale=640:360:force_original_aspect_ratio=decrease:force_divisible_by=2",
		},
		{
			"video filters in commander order",
			`{"video": {"speed": ".5*PTS", "size": "720", "format": "widescreen"},
				"filter": {"deband": true, "deshake": true, "deflicker": true, "dejudder": true,
				"denoise": "light", "deinterlace": "field", "contrast": "1.5", "brightness": "0.2",
				"saturation": "2", "gamma": "0.5"}}`,
			"-vf setpts=.5*PTS,scale=720:-2,deband,deshake,deflicker,dejudder,removegrain=22," +
				"yadif=1:-1:0,eq=contrast=1.5:brightness=0.2:saturation=2:gamma=0.5",
		},
		{
			"denoise default",
			`{"filter": {"denoise": "default"}}`,
			"-vf removegrain=0",
		},
		{
			"empty denoise and deinterlace",
			`{"filter": {"denoise": "", "deinterlace": ""}}`,
			"",
		},
		{
			"audio filters",
			`{"audio": {"codec": "aac", "volume": 150}, "filter": {"acontrast": "50", "adelay": "250"}}`,
			"-c:a aac -af volume=1.5,acontrast=0.5,adelay=delays=250:all=1",
		},
		{
			"clip",
			`{"format": {"clip": true, "startTime": "00:00:05", "stopTime": "00:00:10"}}`,
			"-ss 00:00:05 -to 00:00:10",
		},
		{
			"clip times ignored when clip is off",
			`{"format": {"clip": false, "startTime": "00:00:05"}}`,
			"",
		},
		{
			"booleans as strings",
			`{"video": {"faststart": "true"}, "filter": {"deband": "false"}}`,
			"-movflags faststart",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e, err := NewEncode("in.mp4", "out.mp4", tt.payload)
			if err != nil {
				t.Fatal(err)
			}
			got := strings.Join(optionArgs(t, e.Passes()[0]), " ")
			if got != tt.want {
				t.Errorf("got  %q\nwant %q", got, tt.want)
			}
		})
	}
}

// optionArgs strips the global flags, input and output from a pass.
func optionArgs(t *testing.T, args []string) []string {
	t.Helper()
	i := indexOf(args, "-i")
	if i < 0 || len(args) < i+3 {
		t.Fatalf("no input in %q", args)
	}
	return args[i+2 : len(args)-1]
}

func TestNewEncodeInvalidPayload(t *testing.T) {
	for _, payload := range []string{``, `not json`, `{"video": {"crf": {}}}`, `{"video": {"faststart": 1}}`} {
		if _, err := NewEncode("in.mp4", "out.mp4", payload); err == nil {
			t.Errorf("payload %q: expected an error", payload)
		}
	}
}

func TestNewEncodeGlobalArgs(t *testing.T) {
	e, err := NewEncode("in.mp4", "out.mp4", `{}`)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"-hide_banner", "-nostdin", "-nostats", "-loglevel", "error",
		"-progress", "pipe:1", "-y", "-i", "in.mp4", "out.mp4"}
	if got := e.Passes()[0]; !reflect.DeepEqual(got, want) {
		t.Errorf("got  %q\nwant %q", got, want)
	}
}

func TestNewEncodeRaw(t *testing.T) {
	e, err := NewEncode("in.mp4", "out.mp4", `{"raw": ["-c:v libx264", "-crf 20"]}`)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(optionArgs(t, e.Passes()[0]), " "); got != "-c:v libx264 -crf 20" {
		t.Errorf("got %q", got)
	}
}

func TestTwoPass(t *testing.T) {
	tests := []struct {
		name          string
		payload       string
		first, second string
	}{
		{
			"x264",
			`{"video": {"codec": "libx264", "pass": "2", "bitrate": "2M"}, "audio": {"codec": "aac"}}`,
			"-c:v libx264 -b:v 2M -c:a aac -pass 1 -passlogfile LOG/ffmpeg2pass -an -f null " + os.DevNull,
			"-c:v libx264 -b:v 2M -c:a aac -pass 2 -passlogfile LOG/ffmpeg2pass",
		},
		{
			"x265",
			`{"video": {"codec": "libx265", "pass": "2", "bitrate": "2M"}}`,
			"-c:v libx265 -b:v 2M -x265-params pass=1:stats=LOG/x265.log -an -f null " + os.DevNull,
			"-c:v libx265 -b:v 2M -x265-params pass=2:stats=LOG/x265.log",
		},
		{
			"x265 with codec options",
			`{"video": {"codec": "libx265", "pass": "2", "codec_options": "keyint=48"}}`,
			"-c:v libx265 -x265-params keyint=48:pass=1:stats=LOG/x265.log -an -f null " + os.DevNull,
			"-c:v libx265 -x265-params keyint=48:pass=2:stats=LOG/x265.log",
		},
		{
			"audio already disabled",
			`{"video": {"codec": "libx264", "pass": "2"}, "audio": {"codec": "none"}}`,
			"-c:v libx264 -an -pass 1 -passlogfile LOG/ffmpeg2pass -f null " + os.DevNull,
			"-c:v libx264 -an -pass 2 -passlogfile LOG/ffmpeg2pass",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e, err := NewEncode("in.mp4", "out.mp4", tt.payload)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(e.logDir); err != nil {
				t.Fatalf("log dir not created: %v", err)
			}

			passes := e.Passes()
			if len(passes) != 2 {
				t.Fatalf("got %d passes, want 2", len(passes))
			}
			replace := func(args []string) string {
				s := strings.Join(args, " ")
				s = strings.ReplaceAll(s, e.logDir, "LOG")
				return strings.ReplaceAll(s, "\\", "/")
			}

			// Pass 1 writes to the null muxer, so it has no output file.
			i := indexOf(passes[0], "-i")
			if got := replace(passes[0][i+2:]); got != tt.first {
				t.Errorf("pass 1:\ngot  %q\nwant %q", got, tt.first)
			}
			if got := replace(optionArgs(t, passes[1])); got != tt.second {
				t.Errorf("pass 2:\ngot  %q\nwant %q", got, tt.second)
			}
			if last := passes[1][len(passes[1])-1]; last != "out.mp4" {
				t.Errorf("pass 2 output = %q", last)
			}

			e.Close()
			if _, err := os.Stat(e.logDir); !os.IsNotExist(err) {
				t.Errorf("log dir not removed")
			}
		})
	}
}

func TestOutputDuration(t *testing.T) {
	tests := []struct {
		payload string
		input   float64
		want    float64
	}{
		{`{}`, 60, 60},
		{`{}`, 0, 0},
		{`{"format": {"clip": true, "startTime": "00:00:10", "stopTime": "00:00:40"}}`, 60, 30},
		{`{"format": {"clip": true, "startTime": "10"}}`, 60, 50},
		{`{"format": {"clip": true, "stopTime": "1:30"}}`, 60, 60},
		{`{"format": {"clip": false, "startTime": "10"}}`, 60, 60},
		{`{"video": {"speed": ".5*PTS"}}`, 60, 30},
		{`{"video": {"speed": "2*PTS"}}`, 60, 120},
		{`{"video": {"speed": "auto"}}`, 60, 60},
	}
	for _, tt := range tests {
		e, err := NewEncode("in.mp4", "out.mp4", tt.payload)
		if err != nil {
			t.Fatal(err)
		}
		if got := e.OutputDuration(tt.input); got != tt.want {
			t.Errorf("%s with %vs input: got %v, want %v", tt.payload, tt.input, got, tt.want)
		}
	}
}

func TestParseTime(t *testing.T) {
	tests := map[string]float64{
		"":            0,
		"10":          10,
		"10.5":        10.5,
		"1:30":        90,
		"01:02:03.5":  3723.5,
		"1500ms":      1.5,
		"2s":          2,
		"250000us":    0.25,
		"not a time":  0,
		"00:00:05.25": 5.25,
	}
	for in, want := range tests {
		if got := parseTime(in); got != want {
			t.Errorf("parseTime(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestParseVersion(t *testing.T) {
	tests := map[string]string{
		"ffmpeg version 7.1.1 Copyright (c) 2000-2025":          "7.1.1",
		"ffmpeg version 7.1 Copyright":                          "7.1",
		"ffmpeg version N-118896-g1bb7c2b Copyright":            "N-118896-g1bb7c2b",
		"ffprobe version 9.0.1 Copyright (c) 2007-2026\nsecond": "9.0.1",
		"garbage": "unknown",
	}
	for in, want := range tests {
		if got := parseVersion([]byte(in)); got != want {
			t.Errorf("parseVersion(%q) = %q, want %q", in, got, want)
		}
	}
}
