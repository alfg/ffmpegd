# `ffmpegd`
An FFmpeg server with a websocket API for [FFmpeg Commander](https://github.com/alfg/ffmpeg-commander).

The goal is to provide a simple interface for sending FFmpeg jobs from the browser (and other supported clients in the future) while reporting realtime progress details.

**Currently a work-in-progress! Bugs and breaking changes are expected.*

## How It Works
TODO

## Install
```
$ go get -u github.com/alfg/ffmpegd
```

Release binaries coming soon.

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

Server started on port :8080.
Go to https://alfg.github.io/ffmpeg-commander to connect!

Encoding... 6111 / 17620 (34.68%) 3.37x @ 80.77
```
![ffmpeg-commander](screenshot.png)

## API
TODO

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
