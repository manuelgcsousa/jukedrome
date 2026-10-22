package main

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"syscall"
	"time"
)

var mpvSocket = fmt.Sprintf("/tmp/manuelgcsousa.jukedrome-mpv-%d.sock", os.Getuid())

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
