package ffmpeg

import (
	"testing"
)

const testFile = "../demo/tears-of-steel-5s.mp4"
const testPayload = "{\"format\":{\"container\":\"mp4\",\"clip\":false},\"video\":{\"codec\":\"libx264\",\"preset\":\"none\",\"pass\":\"1\",\"crf\":23,\"pixel_format\":\"auto\",\"frame_rate\":\"auto\",\"speed\":\"auto\",\"tune\":\"none\",\"profile\":\"none\",\"level\":\"none\",\"faststart\":false,\"size\":\"source\",\"width\":\"1080\",\"height\":\"1920\",\"format\":\"widescreen\",\"aspect\":\"auto\",\"scaling\":\"auto\",\"codec_options\":\"\"},\"audio\":{\"codec\":\"copy\",\"channel\":\"source\",\"quality\":\"auto\",\"sampleRate\":\"auto\",\"volume\":\"100\"},\"filter\":{\"deband\":false,\"deshake\":false,\"deflicker\":false,\"dejudder\":false,\"denoise\":\"none\",\"deinterlace\":\"none\",\"brightness\":\"0\",\"contrast\":\"1\",\"saturation\":\"0\",\"gamma\":\"0\",\"acontrast\":\"33\"}}"

func TestFFmpegRun(t *testing.T) {
	f := &FFmpeg{}
	err := f.Run(testFile, "out.mp4", testPayload)
	if err != nil {
		t.Error(err)
	}
}

func TestFFmpegRunFail(t *testing.T) {
	f := &FFmpeg{}
	err := f.Run(testFile, "out.mp4", "{}") // Bad payload.
	if err == nil {
		t.Error()
	}
}
