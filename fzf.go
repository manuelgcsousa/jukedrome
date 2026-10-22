package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

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
