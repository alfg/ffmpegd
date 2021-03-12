# `ffmpegd`
[FFmpeg](https://www.ffmpeg.org/) websocket server and API for [FFmpeg Commander](https://alfg.github.io/ffmpeg-commander).

**Currently a work-in-progress! Bugs and breaking changes are expected.*

## How It Works
`ffmpegd` connects [FFmpeg Commander](https://alfg.github.io/ffmpeg-commander) to [ffmpeg](https://www.ffmpeg.org/) by providing a websocket server to send encode tasks and receive realtime progress updates back to the browser. This allows using `ffmpeg-commander` as a GUI for `ffmpeg`.

The goal is to provide a simple interface for sending FFmpeg tasks from the browser (and other supported clients in the future) to your local machine.

See [Usage](#Usage) for more details.

```
          process              websocket
[ffmpeg] <-------> [ffmpegd] <-----------> [ffmpeg-commander]
```

## Install
```
$ go get -u github.com/alfg/ffmpegd
```

Docker, Homebrew, and release binaries coming soon.

## Usage
* [ffmpeg](https://www.ffmpeg.org/download.html) must be installed and available on your `$PATH`.
* Run `ffmpegd`:
```
$ ffmpegd
```

This wil start the websocket server in your current working directory and wait for a connection.

* Go to https://alfg.github.io/ffmpeg-commander/ in the browser
* Enable `ffmpegd` in Options.
* Once connected, you can start sending encode jobs to ffmpegd!

## Example
### `ffmpegd` with a job in progress from `ffmpeg-commander`
```
$ ffmpegd

███████╗███████╗███╗   ███╗██████╗ ███████╗ ██████╗ ██████╗
██╔════╝██╔════╝████╗ ████║██╔══██╗██╔════╝██╔════╝ ██╔══██╗
█████╗  █████╗  ██╔████╔██║██████╔╝█████╗  ██║  ███╗██║  ██║
██╔══╝  ██╔══╝  ██║╚██╔╝██║██╔═══╝ ██╔══╝  ██║   ██║██║  ██║
██║     ██║     ██║ ╚═╝ ██║██║     ███████╗╚██████╔╝██████╔╝
╚═╝     ╚═╝     ╚═╝     ╚═╝╚═╝     ╚══════╝ ╚═════╝ ╚═════╝
                                                      v0.0.1

[ffmpegd] - websocket server for ffmpeg-commander.

  Checking FFmpeg version....4.3.1
  Checking FFprobe version...4.3.1

  Server started on port :8080.
  - Go to https://alfg.github.io/ffmpeg-commander to connect!
  - ffmpegd must be enabled in ffmpeg-commander options!

Encoding... 6111 / 17620 (34.68%) 3.37x @ 80.77
```
![ffmpeg-commander](screenshot.png)

## API
TBD

## TODO
* Support all `ffmpeg-comamnder` JSON options.
* More CLI flags for server, ports, cwd and daemon mode.
* Logging levels and output
* More error handling
* API documentation
* Docker
* Test Client Demo
* Tests
* Cross-compile binaries for releases

## License
MIT
