# WebSocket Client Demo
To run this demo, build and start `ffmpegd` and load `http://localhost:8080/demo` in your browser.

```
go build -v && ./ffmpegd
```

http://localhost:8080/demo/

## Example
Use the JavaScript WebSocket API to connect and send an encode payload based on the [ffmpeg-commander](https://ffmpeg-commander.com) JSON format:

```javascript
var wsUri = "ws://localhost:8080/ws";
var payload = {
    "format": {
        "container": "mp4",
        "clip": false
    },
    "video": {
        "codec": "libx264",
        "preset": "veryslow",
        "pass": "1",
        "crf": 23,
        "pixel_format": "auto",
        "frame_rate": "auto",
        "speed": "auto",
        "tune": "none",
        "profile": "none",
        "level": "none",
        "faststart": false,
        "size": "source",
        "width": "1080",
        "height": "1920",
        "format": "widescreen",
        "aspect": "auto",
        "scaling": "auto",
        "codec_options": ""
    },
    "audio": {
        "codec": "copy",
        "channel": "source",
        "quality": "auto",
        "sampleRate": "auto",
        "volume": "100"
    },
    "filter": {
        "deband": false,
        "deshake": false,
        "deflicker": false,
        "dejudder": false,
        "denoise": "none",
        "deinterlace": "none",
        "brightness": "0",
        "contrast": "1",
        "saturation": "0",
        "gamma": "0",
        "acontrast": "33"
    }
};
websocket = new WebSocket(wsUri);
websocket.send(JSON.stringify({
    type: 'encode',
    input: 'input.mp4',
    output: 'output.mp4',
    payload: JSON.stringify(payload) // The payload is sent as a JSON string.
}));
```

Paths are relative to the directory `ffmpegd` was started in. Encodes run one at a time, in the order they are sent.

## Responses
The server sends progress about once a second while encoding:

```JSON
{"percent":34.68,"speed":"3.37x","fps":80.77}
```

Progress is measured against the length of the output, so it works for audio-only encodes and clips. For two-pass encodes, each pass is half of the total. `percent` stays below 100 until the encode is finished.

When the encode finishes:

```JSON
{"percent":100,"speed":"","fps":0}
```

If the encode fails, or a message can't be read, `err` holds the reason, usually ffmpeg's error output:

```JSON
{"percent":0,"speed":"","fps":0,"err":"Unknown encoder 'libaom-av1'"}
```

## Cancelling
Stop the running encode with:

```javascript
websocket.send(JSON.stringify({ type: 'cancel' }));
```

The server kills ffmpeg and replies:

```JSON
{"percent":0,"speed":"","fps":0,"cancelled":true}
```