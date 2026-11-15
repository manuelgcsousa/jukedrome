# jukedrome

A tiny fzf-driven jukebox for [Navidrome](https://www.navidrome.org/).

It lets you browse your music library (artists ~> albums ~> songs) with `fzf`.
Picking a track queues the whole album in `mpv`, starting from that track.

## Requirements

- A running Navidrome server
- [go](https://go.dev/dl/) (for building with your configuration details)
- [just](https://github.com/casey/just)
- [fzf](https://github.com/junegunn/fzf)
- [mpv](https://mpv.io/)

## Setup

Adapt `config.go` with your details (à la suckless). The build will create it from `config.go.example` if it does not exist yet:

```go
serverUrl = "http://127.0.0.1:4533"
username  = "your-username"
password  = "your-password"

fzfBin = "fzf"
mpvBin = "mpv"

fzfRefreshKey = "ctrl-r"
```

## Build & run

```sh
just run
```

## Usage

```sh
jukedrome --help
```

Pick a track and the whole album gets queued, starting from there. If an `mpv` instance started by `jukedrome` is already running, it reuses that instance; otherwise, it starts a new one.
