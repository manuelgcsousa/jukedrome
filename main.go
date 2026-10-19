package main

import (
	"bytes"
	"crypto/md5"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"
)

var (
	errAborted = errors.New("aborted")
	errNoMatch = errors.New("no match")

	mpvSocket = fmt.Sprintf("/tmp/jukedrome-%d-mpv.sock", os.Getuid())
)

type item struct {
	ID    string
	Label string
	Crumb string
}

type subsonicResponse struct {
	Status  string         `json:"status"`
	Error   *subsonicError `json:"error"`
	Artists *struct {
		Index []struct {
			Artist []struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"artist"`
		} `json:"index"`
	} `json:"artists"`
	Artist *struct {
		Album []struct {
			ID        string `json:"id"`
			Name      string `json:"name"`
			Year      int    `json:"year"`
			SongCount int    `json:"songCount"`
		} `json:"album"`
	} `json:"artist"`
	Album *struct {
		Song []struct {
			ID    string `json:"id"`
			Title string `json:"title"`
			Track int    `json:"track"`
		} `json:"song"`
	} `json:"album"`
}

type subsonicError struct {
	Message string `json:"message"`
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

func fzf(items []item, prompt string) (item, bool, error) {
	var input strings.Builder
	for _, it := range items {
		fmt.Fprintf(&input, "%s\t%s\n", it.ID, it.Label)
	}

	args := []string{
		"--reverse",
		"--border",
		"--height=50%",
		"--delimiter=\t",
		"--with-nth=2", // only render field nº2 (it.Label)
		"--prompt=" + prompt,
		"--expect=" + fzfRefreshKey,
	}

	cmd := exec.Command(fzfBin, args...)
	cmd.Stdin = strings.NewReader(input.String())
	cmd.Stderr = os.Stderr
	var output bytes.Buffer
	cmd.Stdout = &output

	if err := cmd.Run(); err != nil {
		if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
			switch exitErr.ExitCode() {
			case 130:
				return item{}, false, errAborted
			case 1:
				return item{}, false, errNoMatch
			}
		}
		return item{}, false, err
	}

	lines := strings.Split(strings.TrimRight(output.String(), "\n"), "\n")
	if len(lines) > 0 && lines[0] == fzfRefreshKey {
		return item{}, true, nil
	}
	id, _, _ := strings.Cut(lines[len(lines)-1], "\t")
	for _, it := range items {
		if it.ID == id {
			return it, false, nil
		}
	}
	return item{}, false, errNoMatch
}

func getArtists() ([]item, error) {
	resp, err := subsonicApiRequest("getArtists", nil)
	if err != nil {
		return nil, err
	}

	var items []item
	if resp.Artists != nil {
		for _, index := range resp.Artists.Index {
			for _, artist := range index.Artist {
				items = append(items, item{ID: artist.ID, Label: artist.Name, Crumb: artist.Name})
			}
		}
	}
	return items, nil
}

func getAlbums(artistID string) ([]item, error) {
	resp, err := subsonicApiRequest("getArtist", map[string]string{"id": artistID})
	if err != nil {
		return nil, err
	}

	var items []item
	if resp.Artist != nil {
		for _, album := range resp.Artist.Album {
			year := "????"
			if album.Year > 0 {
				year = strconv.Itoa(album.Year)
			}
			label := fmt.Sprintf("%s (%s) · %d songs", album.Name, year, album.SongCount)
			crumb := fmt.Sprintf("%s (%s)", album.Name, year)
			items = append(items, item{ID: album.ID, Label: label, Crumb: crumb})
		}
	}
	return items, nil
}

func getSongs(albumID string) ([]item, error) {
	resp, err := subsonicApiRequest("getAlbum", map[string]string{"id": albumID})
	if err != nil {
		return nil, err
	}

	var items []item
	if resp.Album != nil {
		for _, song := range resp.Album.Song {
			track := "?"
			if song.Track > 0 {
				track = strconv.Itoa(song.Track)
			}
			items = append(items, item{ID: song.ID, Label: fmt.Sprintf("%s. %s", track, song.Title)})
		}
	}
	return items, nil
}

func subsonicApiRequest(endpoint string, params map[string]string) (*subsonicResponse, error) {
	salt := randomSalt()
	sum := md5.Sum([]byte(password + salt))
	query := url.Values{
		"u": {username},
		"t": {hex.EncodeToString(sum[:])},
		"s": {salt},
		"v": {apiVersion},
		"c": {clientName},
		"f": {"json"},
	}
	for key, value := range params {
		query.Set(key, value)
	}

	resp, err := http.Get(serverUrl + "/rest/" + endpoint + "?" + query.Encode())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: HTTP %d", endpoint, resp.StatusCode)
	}

	var envelope struct {
		Response subsonicResponse `json:"subsonic-response"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, fmt.Errorf("%s: %w", endpoint, err)
	}
	if envelope.Response.Status != "ok" {
		message := "unknown API error"
		if envelope.Response.Error != nil {
			message = envelope.Response.Error.Message
		}
		return nil, errors.New(message)
	}
	return &envelope.Response, nil
}

func streamUrl(songID string) string {
	salt := randomSalt()
	sum := md5.Sum([]byte(password + salt))
	query := url.Values{
		"u":  {username},
		"t":  {hex.EncodeToString(sum[:])},
		"s":  {salt},
		"v":  {apiVersion},
		"c":  {clientName},
		"id": {songID},
	}
	return serverUrl + "/rest/stream?" + query.Encode()
}

func play(links []string, start int) {
	if len(links) == 0 {
		return
	}
	if loadIntoRunningMpv(links, start) {
		return
	}
	_ = os.Remove(mpvSocket)

	args := make([]string, 0, len(links)+2)
	args = append(args, "--input-ipc-server="+mpvSocket)
	args = append(args, fmt.Sprintf("--playlist-start=%d", start))
	args = append(args, links...)

	cmd := exec.Command(mpvBin, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "jukedrome: could not start mpv: %v\n", err)
		return
	}
	_ = cmd.Process.Release()
}

func loadIntoRunningMpv(links []string, start int) bool {
	conn, err := net.Dial("unix", mpvSocket)
	if err != nil {
		return false
	}
	defer conn.Close()

	_ = conn.SetDeadline(time.Now().Add(2 * time.Second))
	enc := json.NewEncoder(conn)

	if err := enc.Encode(map[string]any{
		"command": []string{"loadfile", links[0], "replace"},
	}); err != nil {
		return false
	}
	for _, link := range links[1:] {
		if err := enc.Encode(map[string]any{
			"command": []string{"playlist-append", link},
		}); err != nil {
			return false
		}
	}
	if start > 0 {
		if err := enc.Encode(map[string]any{
			"command": []any{"set_property", "playlist-pos", start},
		}); err != nil {
			return false
		}
	}

	return true
}

func randomSalt() string {
	buf := make([]byte, 8)
	_, _ = rand.Read(buf)
	return hex.EncodeToString(buf)
}
