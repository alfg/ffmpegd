# WebSocket Client Demo
To run this demo, build and start `ffmpegd` and load `http://localhost:8080/demo` in your browser.

```
go build -v && ./ffmpegd
```

http://localhost:8080/demo/

## Example
Use the JavaScript WebSocket API to connect and send an encode payload based on the [ffmpeg-commander](https://alfg.github.io/ffmpeg-commander) JSON format:

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
    payload: payload
}));
```

The websocket server will respond with progress until the encode is complete.

```JSON
{"percent":59.17,"speed":"5.31x","fps":0}

{"percent":95,"speed":"2.98x","fps":67.87}

{"percent":95,"speed":"2.98x","fps":67.87}

{"percent":100,"speed":"1.29x","fps":31.04}

{"percent":100,"speed":"","fps":0} 
```