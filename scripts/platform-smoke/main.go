// Command platform-smoke verifies an installed Sloth binary on its native OS.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/ansonnchan/sloth/internal/report"
)

func main() {
	binary := flag.String("binary", "", "path to the native Sloth binary")
	version := flag.String("version", "", "expected version")
	commit := flag.String("commit", "", "expected commit")
	target := flag.String("target", "", "expected GOOS-GOARCH target")
	flag.Parse()
	if err := run(*binary, *version, *commit, *target); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(binary, expectedVersion, expectedCommit, expectedTarget string) error {
	if binary == "" || expectedVersion == "" || expectedCommit == "" || expectedTarget == "" {
		return fmt.Errorf("binary, version, commit, and target are required")
	}
	if target := runtime.GOOS + "-" + runtime.GOARCH; target != expectedTarget {
		return fmt.Errorf("smoke harness target = %s, want %s", target, expectedTarget)
	}
	absoluteBinary, err := filepath.Abs(binary)
	if err != nil {
		return fmt.Errorf("resolve binary: %w", err)
	}
	if _, err := os.Stat(absoluteBinary); err != nil {
		return fmt.Errorf("inspect binary: %w", err)
	}

	if result, err := execute(absoluteBinary, "--help"); err != nil || !strings.Contains(result.stdout, "Sloth explains") {
		return fmt.Errorf("help smoke failed: %w\nstdout: %s\nstderr: %s", err, result.stdout, result.stderr)
	}
	if result, err := execute(absoluteBinary, "--version"); err != nil || !strings.Contains(result.stdout, expectedVersion) || !strings.Contains(result.stdout, expectedCommit) {
		return fmt.Errorf("version smoke failed: %w\nstdout: %s\nstderr: %s", err, result.stdout, result.stderr)
	}

	server, err := startServer()
	if err != nil {
		return err
	}
	defer server.close()
	if result, err := execute(absoluteBinary, "inspect", "--json", server.url); err != nil {
		return fmt.Errorf("inspect smoke failed: %w\nstdout: %s\nstderr: %s", err, result.stdout, result.stderr)
	} else {
		var document struct {
			SchemaVersion string `json:"schema_version"`
		}
		if err := json.Unmarshal([]byte(result.stdout), &document); err != nil || document.SchemaVersion != "sloth.inspect/v1" {
			return fmt.Errorf("inspect JSON smoke failed: schema=%q error=%v", document.SchemaVersion, err)
		}
	}

	directory, err := os.MkdirTemp("", "sloth-platform-smoke-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(directory)
	path := filepath.Join(directory, "report.json")
	exportArgs := []string{
		"export", "--quiet", "--output", path, "--requests", "10", "--warmups", "0",
		"--rate", "100", "--concurrency", "1,2", server.url,
	}
	if result, err := execute(absoluteBinary, exportArgs...); err != nil {
		return fmt.Errorf("export smoke failed: %w\nstdout: %s\nstderr: %s", err, result.stdout, result.stderr)
	}
	if err := validateReport(path, expectedVersion, expectedCommit); err != nil {
		return err
	}
	original, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if result, err := execute(absoluteBinary, exportArgs...); err == nil {
		return fmt.Errorf("exclusive report smoke unexpectedly succeeded: %s", result.stdout)
	}
	afterExclusiveFailure, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(original, afterExclusiveFailure) {
		return fmt.Errorf("exclusive report smoke changed the existing file: %v", err)
	}
	forceArgs := append([]string{"export", "--force"}, exportArgs[1:]...)
	if result, err := execute(absoluteBinary, forceArgs...); err != nil {
		return fmt.Errorf("forced report smoke failed: %w\nstdout: %s\nstderr: %s", err, result.stdout, result.stderr)
	}
	if err := validateReport(path, expectedVersion, expectedCommit); err != nil {
		return err
	}
	entries, err := os.ReadDir(directory)
	if err != nil || len(entries) != 1 || entries[0].Name() != "report.json" {
		return fmt.Errorf("report directory contains unexpected files: entries=%v error=%v", entries, err)
	}
	return nil
}

type commandResult struct {
	stdout string
	stderr string
}

func execute(binary string, arguments ...string) (commandResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, binary, arguments...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	err := command.Run()
	if ctx.Err() != nil {
		err = ctx.Err()
	}
	return commandResult{stdout: stdout.String(), stderr: stderr.String()}, err
}

func validateReport(path, expectedVersion, expectedCommit string) error {
	parsed, err := report.Read(path)
	if err != nil {
		return fmt.Errorf("read exported report: %w", err)
	}
	if parsed.Report.ToolVersion != expectedVersion || parsed.Report.Tool.Commit != expectedCommit {
		return fmt.Errorf("report metadata = %s/%s, want %s/%s", parsed.Report.ToolVersion, parsed.Report.Tool.Commit, expectedVersion, expectedCommit)
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(path)
		if err != nil {
			return err
		}
		if permission := info.Mode().Perm(); permission != 0o600 {
			return fmt.Errorf("report permission = %04o, want 0600", permission)
		}
	}
	return nil
}

type localServer struct {
	server   *http.Server
	listener net.Listener
	url      string
}

func startServer() (*localServer, error) {
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("start local endpoint: %w", err)
	}
	server := &http.Server{Handler: http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte("{\"ok\":true}\n"))
	}), ReadHeaderTimeout: 5 * time.Second}
	go func() { _ = server.Serve(listener) }()
	return &localServer{server: server, listener: listener, url: "http://" + listener.Addr().String()}, nil
}

func (server *localServer) close() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = server.server.Shutdown(ctx)
	_ = server.listener.Close()
}
