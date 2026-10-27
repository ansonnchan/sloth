package report

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestResolveOutputPathUsesExistingDesktopAndSafeUniqueName(t *testing.T) {
	home := t.TempDir()
	desktop := filepath.Join(home, "Desktop")
	if err := os.Mkdir(desktop, 0o700); err != nil {
		t.Fatal(err)
	}
	path, err := ResolveOutputPath(PathOptions{
		HomeDir: func() (string, error) { return home, nil },
		Now:     func() time.Time { return time.Date(2026, 1, 2, 3, 4, 5, 0, time.FixedZone("local", 3600)) },
		Random:  bytes.NewReader([]byte{0xde, 0xad, 0xbe, 0xef}),
	})
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(desktop, "sloth-20260102T020405Z-deadbeef.json")
	if path != want {
		t.Fatalf("path = %q, want %q", path, want)
	}
}

func TestResolveOutputPathDoesNotCreateMissingDesktop(t *testing.T) {
	home := t.TempDir()
	_, err := ResolveOutputPath(PathOptions{HomeDir: func() (string, error) { return home, nil }})
	if err == nil || !strings.Contains(err.Error(), "does not exist") || !strings.Contains(err.Error(), "--output") {
		t.Fatalf("error = %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(home, "Desktop")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("Desktop was created: %v", statErr)
	}
}

func TestResolveOutputPathResolvesRelativeAgainstWorkingDirectory(t *testing.T) {
	working := t.TempDir()
	path, err := ResolveOutputPath(PathOptions{
		Output:     filepath.Join("reports", "run.json"),
		WorkingDir: func() (string, error) { return working, nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(working, "reports", "run.json"); path != want {
		t.Fatalf("path = %q, want %q", path, want)
	}
}

func TestWriteAtomicPublishesCompleteFileWithoutOverwrite(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "report.json")
	if err := WriteAtomic(path, []byte("first"), false); err != nil {
		t.Fatal(err)
	}
	if err := WriteAtomic(path, []byte("second"), false); !errors.Is(err, ErrOutputExists) {
		t.Fatalf("error = %v", err)
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(contents) != "first" {
		t.Fatalf("contents = %q", contents)
	}
	assertNoTemporaryReports(t, directory)
}

func TestWriteAtomicForceReplacesExistingFile(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "report.json")
	if err := os.WriteFile(path, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := WriteAtomic(path, []byte("new"), true); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(contents) != "new" {
		t.Fatalf("contents = %q", contents)
	}
	assertNoTemporaryReports(t, directory)
}

func TestWriteAtomicFailureLeavesNoFinalOrTemporaryFile(t *testing.T) {
	directory := t.TempDir()
	missingParent := filepath.Join(directory, "missing")
	path := filepath.Join(missingParent, "report.json")
	if err := WriteAtomic(path, []byte("data"), false); err == nil || !strings.Contains(err.Error(), "destination directory") {
		t.Fatalf("error = %v", err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("final path exists: %v", err)
	}
	assertNoTemporaryReports(t, directory)
}

func TestWriteAtomicDestinationMustBeDirectory(t *testing.T) {
	directory := t.TempDir()
	parent := filepath.Join(directory, "not-a-directory")
	if err := os.WriteFile(parent, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	err := WriteAtomic(filepath.Join(parent, "report.json"), []byte("data"), false)
	if err == nil || !strings.Contains(err.Error(), "destination") {
		t.Fatalf("error = %v", err)
	}
}

func assertNoTemporaryReports(t *testing.T, directory string) {
	t.Helper()
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".sloth-report-") {
			t.Fatalf("temporary file remains: %s", entry.Name())
		}
	}
}
