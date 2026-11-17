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
	if !stopRunningMpv() {
		return
	}

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

func stopRunningMpv() bool {
	conn, err := net.DialTimeout("unix", mpvSocket, 200*time.Millisecond)
	if err != nil {
		_ = os.Remove(mpvSocket)
		return true
	}

	_ = conn.SetDeadline(time.Now().Add(time.Second))
	if err := json.NewEncoder(conn).Encode(map[string]any{
		"command": []string{"quit"},
	}); err != nil {
		fmt.Fprintf(os.Stderr, "jukedrome: could not stop mpv: %v\n", err)
		_ = conn.Close()
		return false
	}
	_ = conn.Close()

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("unix", mpvSocket, 100*time.Millisecond)
		if err != nil {
			_ = os.Remove(mpvSocket)
			return true
		}
		_ = conn.Close()
		time.Sleep(50 * time.Millisecond)
	}

	fmt.Fprintln(os.Stderr, "jukedrome: timed out waiting for mpv to stop")
	return false
}
