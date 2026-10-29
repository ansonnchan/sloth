// Command release-verify validates GoReleaser's complete Sloth artifact set.
package main

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/ansonnchan/sloth/internal/releasecheck"
)

type target struct {
	goos      string
	goarch    string
	extension string
	binary    string
}

var targets = []target{
	{goos: "darwin", goarch: "amd64", extension: ".tar.gz", binary: "sloth"},
	{goos: "darwin", goarch: "arm64", extension: ".tar.gz", binary: "sloth"},
	{goos: "linux", goarch: "amd64", extension: ".tar.gz", binary: "sloth"},
	{goos: "linux", goarch: "arm64", extension: ".tar.gz", binary: "sloth"},
	{goos: "windows", goarch: "amd64", extension: ".zip", binary: "sloth.exe"},
}

func main() {
	dist := flag.String("dist", "./dist", "GoReleaser output directory")
	commit := flag.String("commit", "", "expected full commit SHA")
	flag.Parse()
	if err := verify(*dist, *commit); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func verify(dist, expectedCommit string) error {
	if expectedCommit == "" {
		return errors.New("commit is required")
	}
	absoluteDist, err := filepath.Abs(dist)
	if err != nil {
		return err
	}
	checksums, err := readChecksums(filepath.Join(absoluteDist, "checksums.txt"))
	if err != nil {
		return err
	}
	version := ""
	for _, target := range targets {
		pattern := filepath.Join(absoluteDist, "sloth_*_"+target.goos+"_"+target.goarch+target.extension)
		matches, err := filepath.Glob(pattern)
		if err != nil || len(matches) != 1 {
			return fmt.Errorf("archive pattern %q matched %d files: %v", pattern, len(matches), err)
		}
		path := matches[0]
		archiveVersion, err := versionFromArchive(path, target)
		if err != nil {
			return err
		}
		if version == "" {
			version = archiveVersion
		} else if archiveVersion != version {
			return fmt.Errorf("archive %s has version %q, want %q", filepath.Base(path), archiveVersion, version)
		}
		if err := verifyChecksum(path, checksums); err != nil {
			return err
		}
		files, binary, err := readArchive(path, target)
		if err != nil {
			return err
		}
		for _, required := range []string{target.binary, "LICENSE", "README.md", "CHANGELOG.md"} {
			if !files[required] {
				return fmt.Errorf("archive %s is missing %s", filepath.Base(path), required)
			}
		}
		if target.goos == runtime.GOOS && target.goarch == runtime.GOARCH {
			if err := verifyNativeBinary(binary, target.binary, version, expectedCommit); err != nil {
				return err
			}
		}
	}
	if len(checksums) != len(targets) {
		return fmt.Errorf("checksums.txt has %d entries, want %d", len(checksums), len(targets))
	}
	return nil
}

func versionFromArchive(path string, target target) (string, error) {
	name := filepath.Base(path)
	prefix := "sloth_"
	suffix := "_" + target.goos + "_" + target.goarch + target.extension
	if !strings.HasPrefix(name, prefix) || !strings.HasSuffix(name, suffix) {
		return "", fmt.Errorf("unexpected archive name %q", name)
	}
	version := strings.TrimSuffix(strings.TrimPrefix(name, prefix), suffix)
	if version == "" || strings.ContainsAny(version, `/\\`) {
		return "", fmt.Errorf("invalid archive version in %q", name)
	}
	return version, nil
}

func readChecksums(path string) (map[string]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open checksums: %w", err)
	}
	defer file.Close()
	values := make(map[string]string)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) != 2 {
			return nil, fmt.Errorf("invalid checksum line %q", scanner.Text())
		}
		if _, err := hex.DecodeString(fields[0]); err != nil || len(fields[0]) != sha256.Size*2 {
			return nil, fmt.Errorf("invalid SHA-256 checksum %q", fields[0])
		}
		values[filepath.Base(fields[1])] = strings.ToLower(fields[0])
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return values, nil
}

func verifyChecksum(path string, checksums map[string]string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	digest := sha256.New()
	if _, err := io.Copy(digest, file); err != nil {
		return err
	}
	actual := hex.EncodeToString(digest.Sum(nil))
	expected, ok := checksums[filepath.Base(path)]
	if !ok || actual != expected {
		return fmt.Errorf("checksum mismatch for %s", filepath.Base(path))
	}
	return nil
}

func readArchive(path string, target target) (map[string]bool, []byte, error) {
	if target.extension == ".zip" {
		return readZip(path, target.binary)
	}
	return readTarGz(path, target.binary)
}

func readZip(path, binaryName string) (map[string]bool, []byte, error) {
	archive, err := zip.OpenReader(path)
	if err != nil {
		return nil, nil, err
	}
	defer archive.Close()
	files := make(map[string]bool)
	var binary []byte
	for _, entry := range archive.File {
		name := filepath.Base(entry.Name)
		files[name] = true
		if name != binaryName {
			continue
		}
		reader, err := entry.Open()
		if err != nil {
			return nil, nil, err
		}
		binary, err = io.ReadAll(reader)
		reader.Close()
		if err != nil {
			return nil, nil, err
		}
	}
	return files, binary, nil
}

func readTarGz(path, binaryName string) (map[string]bool, []byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	defer file.Close()
	compressed, err := gzip.NewReader(file)
	if err != nil {
		return nil, nil, err
	}
	defer compressed.Close()
	archive := tar.NewReader(compressed)
	files := make(map[string]bool)
	var binary []byte
	for {
		header, err := archive.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, nil, err
		}
		name := filepath.Base(header.Name)
		files[name] = true
		if name == binaryName {
			binary, err = io.ReadAll(archive)
			if err != nil {
				return nil, nil, err
			}
		}
	}
	return files, binary, nil
}

func verifyNativeBinary(binary []byte, name, version, commit string) error {
	if len(binary) == 0 {
		return errors.New("native archive has no binary data")
	}
	directory, err := os.MkdirTemp("", "sloth-release-verify-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(directory)
	path := filepath.Join(directory, name)
	if err := os.WriteFile(path, binary, 0o700); err != nil {
		return err
	}
	return releasecheck.RunPlatformSmoke(path, version, commit, runtime.GOOS+"-"+runtime.GOARCH)
}
