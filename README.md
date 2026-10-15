# jukedrome

A tiny fzf-driven jukebox for [Navidrome](https://www.navidrome.org/).

It lets you browse your music (artists --> albums --> songs) with `fzf`. Picking a track queues the whole album in `mpv`, starting from that track.

## Requirements

- Go
- [fzf](https://github.com/junegunn/fzf)
- [mpv](https://mpv.io/)
- A running Navidrome server

## Setup

Adapt `config.go` with your details (the build will create it from `config.go.example` if it does not exist yet):

```go
serverUrl = "http://127.0.0.1:4533"
username  = "your-username"
password  = "your-password"

fzfBin = "fzf"
mpvBin = "mpv"
```

## Build & run

```sh
just run
```

## Usage

Run `jukedrome` and navigate:

- Type to filter
- Enter to select
- Esc to go back (quits when at the artist list)

Pick a track and the whole album gets queued, starting from there. If `mpv` is already running, `jukedrome` reuses that instance.
