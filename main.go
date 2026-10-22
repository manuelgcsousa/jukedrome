package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
)

var (
	errAborted = errors.New("aborted")
	errNoMatch = errors.New("no match")
)

type item struct {
	ID    string
	Label string
	Crumb string
}

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "-h", "--help":
			printUsage()
			return
		default:
			fmt.Fprintf(os.Stderr, "jukedrome: unknown argument %q\n", os.Args[1])
			printUsage()
			os.Exit(2)
		}
	}

	for _, bin := range []string{fzfBin, mpvBin} {
		if _, err := exec.LookPath(bin); err != nil {
			fmt.Fprintf(os.Stderr, "jukedrome: %q not found in PATH\n", bin)
			os.Exit(1)
		}
	}

	if err := browse(); err != nil {
		fmt.Fprintf(os.Stderr, "jukedrome: %v\n", err)
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Print(`jukedrome - fzf-driven Navidrome jukebox

Usage:
  jukedrome         Browse artists > albums > tracks and play
  jukedrome --help  Show this help

Keys:
  Enter       Select track
  Esc/Ctrl-C  Go back (quits at artists)
  Ctrl-R      Refetch section
  Track       Play album from selected track
`)
}

func browse() error {
	var (
		artists []item
		albums  = map[string][]item{}
		songs   = map[string][]item{}
	)

	for {
		if artists == nil {
			loaded, err := getArtists()
			if err != nil {
				return err
			}
			artists = loaded
		}
		artist, refresh, err := fzf(artists, "Artist › ")
		if errors.Is(err, errAborted) {
			return nil
		}
		if errors.Is(err, errNoMatch) {
			continue
		}
		if err != nil {
			return err
		}
		if refresh {
			artists = nil
			albums = map[string][]item{}
			songs = map[string][]item{}
			continue
		}

		for {
			cached, ok := albums[artist.ID]
			if !ok {
				loaded, err := getAlbums(artist.ID)
				if err != nil {
					return err
				}
				cached = loaded
				albums[artist.ID] = cached
			}
			album, refresh, err := fzf(cached, fmt.Sprintf("%s › ", artist.Crumb))
			if errors.Is(err, errAborted) {
				break
			}
			if errors.Is(err, errNoMatch) {
				continue
			}
			if err != nil {
				return err
			}
			if refresh {
				for _, a := range albums[artist.ID] {
					delete(songs, a.ID)
				}
				delete(albums, artist.ID)
				continue
			}

			for {
				cachedSongs, ok := songs[album.ID]
				if !ok {
					loaded, err := getSongs(album.ID)
					if err != nil {
						return err
					}
					cachedSongs = loaded
					songs[album.ID] = cachedSongs
				}
				song, refresh, err := fzf(cachedSongs, fmt.Sprintf("%s › %s › ", artist.Crumb, album.Crumb))
				if errors.Is(err, errAborted) {
					break
				}
				if errors.Is(err, errNoMatch) {
					continue
				}
				if err != nil {
					return err
				}
				if refresh {
					delete(songs, album.ID)
					continue
				}

				links := make([]string, len(cachedSongs))
				start := 0
				for i, s := range cachedSongs {
					links[i] = streamUrl(s.ID)
					if s.ID == song.ID {
						start = i
					}
				}
				play(links, start)
				return nil
			}
		}
	}
}
