// Command demo-record captures real release-candidate output as asciinema v2.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/ansonnchan/sloth/examples/sample-api/sampleapi"
)

type castEvent struct {
	at   float64
	kind string
	text string
}

func main() {
	binary := flag.String("binary", "", "release-candidate Sloth binary")
	output := flag.String("output", "docs/demo/sloth.cast", "asciinema v2 output")
	flag.Parse()
	if err := record(*binary, *output); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func record(binary, output string) error {
	if binary == "" {
		return errors.New("binary is required")
	}
	absoluteBinary, err := filepath.Abs(binary)
	if err != nil {
		return err
	}
	listener, err := net.Listen("tcp4", "127.0.0.1:8080")
	if err != nil {
		return fmt.Errorf("listen on demo endpoint 127.0.0.1:8080: %w", err)
	}
	server := &http.Server{Handler: sampleapi.NewHandler(), ReadHeaderTimeout: 5 * time.Second}
	go func() { _ = server.Serve(listener) }()
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(ctx)
	}()

	commands := []struct {
		display string
		args    []string
	}{
		{display: "sloth --version", args: []string{"--version"}},
		{display: "sloth inspect --ascii --color never http://127.0.0.1:8080/delay?ms=120", args: []string{"inspect", "--ascii", "--color", "never", "http://127.0.0.1:8080/delay?ms=120"}},
		{display: "sloth diagnose --ascii --color never --requests 10 --warmups 0 --rate 100 --concurrency 1,2 http://127.0.0.1:8080/concurrency?delay_ms=30", args: []string{"diagnose", "--ascii", "--color", "never", "--requests", "10", "--warmups", "0", "--rate", "100", "--concurrency", "1,2", "http://127.0.0.1:8080/concurrency?delay_ms=30"}},
	}

	events := []castEvent{{at: 0.2, kind: "o", text: "Sloth v1 release-candidate demo (local deterministic endpoint)\r\n\r\n"}}
	clock := 0.8
	for _, command := range commands {
		events = append(events, castEvent{at: clock, kind: "o", text: "$ " + command.display + "\r\n"})
		result, err := execute(absoluteBinary, command.args...)
		if err != nil {
			return fmt.Errorf("run %s: %w\n%s", command.display, err, result)
		}
		clock += 0.45
		events = append(events, castEvent{at: clock, kind: "o", text: terminalLines(result)})
		clock += 1.2
	}
	events = append(events, castEvent{at: clock, kind: "o", text: "\r\nMetrics above are genuine local observations and illustrative only.\r\n"})
	return writeCast(output, events)
}

func execute(binary string, arguments ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, binary, arguments...)
	command.Env = append(os.Environ(), "NO_COLOR=1")
	var output bytes.Buffer
	command.Stdout = &output
	command.Stderr = &output
	err := command.Run()
	if ctx.Err() != nil {
		err = ctx.Err()
	}
	return output.String(), err
}

func terminalLines(value string) string {
	value = strings.ReplaceAll(value, "\r\n", "\n")
	value = strings.ReplaceAll(value, "\n", "\r\n")
	return value
}

func writeCast(path string, events []castEvent) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	var output bytes.Buffer
	header := map[string]any{
		"version": 2, "width": 108, "height": 34,
		"timestamp": time.Now().UTC().Unix(),
		"title":     "Sloth v1 release-candidate demo",
		"env":       map[string]string{"SHELL": "/bin/sh", "TERM": "xterm-256color"},
	}
	encodedHeader, _ := json.Marshal(header)
	output.Write(encodedHeader)
	output.WriteByte('\n')
	for _, event := range events {
		encoded, _ := json.Marshal([]any{event.at, event.kind, event.text})
		output.Write(encoded)
		output.WriteByte('\n')
	}
	return os.WriteFile(path, output.Bytes(), 0o644)
}
