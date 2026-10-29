package report

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

const defaultReportPrefix = "sloth"

var ErrOutputExists = errors.New("report output already exists")

// PathOptions controls report destination resolution. Empty Output selects the
// current user's Desktop without creating it.
type PathOptions struct {
	Output     string
	Now        func() time.Time
	Random     io.Reader
	HomeDir    func() (string, error)
	WorkingDir func() (string, error)
}

// ResolveOutputPath returns an absolute output path. The default path is a
// collision-resistant JSON filename on the existing Desktop directory.
func ResolveOutputPath(options PathOptions) (string, error) {
	if options.Output != "" {
		path := options.Output
		if !filepath.IsAbs(path) {
			workingDir := options.WorkingDir
			if workingDir == nil {
				workingDir = os.Getwd
			}
			base, err := workingDir()
			if err != nil {
				return "", fmt.Errorf("resolve working directory: %w", err)
			}
			path = filepath.Join(base, path)
		}
		absolute, err := filepath.Abs(filepath.Clean(path))
		if err != nil {
			return "", fmt.Errorf("resolve output path %q: %w", options.Output, err)
		}
		return absolute, nil
	}

	homeDir := options.HomeDir
	if homeDir == nil {
		homeDir = os.UserHomeDir
	}
	home, err := homeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory for default output: %w", err)
	}
	desktop := filepath.Join(home, "Desktop")
	info, err := os.Stat(desktop)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("default Desktop directory %q does not exist; pass --output: %w", desktop, err)
		}
		return "", fmt.Errorf("inspect default Desktop directory %q: %w", desktop, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("default Desktop path %q is not a directory; pass --output", desktop)
	}

	now := options.Now
	if now == nil {
		now = time.Now
	}
	random := options.Random
	if random == nil {
		random = rand.Reader
	}
	suffix := make([]byte, 4)
	if _, err := io.ReadFull(random, suffix); err != nil {
		return "", fmt.Errorf("generate report filename suffix: %w", err)
	}
	name := fmt.Sprintf("%s-%s-%s.json", defaultReportPrefix, now().UTC().Format("20060102T150405Z"), hex.EncodeToString(suffix))
	return filepath.Join(desktop, name), nil
}

// WriteAtomic publishes data only after it has been fully written, synced, and
// closed. Existing files are preserved unless force is explicit.
func WriteAtomic(path string, data []byte, force bool) (returnErr error) {
	path = filepath.Clean(path)
	directory := filepath.Dir(path)
	info, err := os.Stat(directory)
	if err != nil {
		return fmt.Errorf("inspect report destination directory %q: %w", directory, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("report destination %q is not a directory", directory)
	}
	if !force {
		if _, err := os.Lstat(path); err == nil {
			return fmt.Errorf("%w: %s (pass --force to replace it)", ErrOutputExists, path)
		} else if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("inspect report output %q: %w", path, err)
		}
	}

	temporary, err := os.CreateTemp(directory, ".sloth-report-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary report in %q: %w", directory, err)
	}
	temporaryPath := temporary.Name()
	defer func() {
		if temporary != nil {
			_ = temporary.Close()
		}
		if err := os.Remove(temporaryPath); err != nil && !errors.Is(err, os.ErrNotExist) && returnErr == nil {
			returnErr = fmt.Errorf("clean temporary report %q: %w", temporaryPath, err)
		}
	}()

	if _, err := temporary.Write(data); err != nil {
		return fmt.Errorf("write temporary report: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		return fmt.Errorf("sync temporary report: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close temporary report: %w", err)
	}
	temporary = nil

	if force {
		if err := os.Rename(temporaryPath, path); err != nil {
			return fmt.Errorf("replace report output %q: %w", path, err)
		}
	} else {
		// Linking is an atomic exclusive publish: it cannot replace a target
		// created after the preflight check, and readers never see partial data.
		if err := os.Link(temporaryPath, path); err != nil {
			if _, statErr := os.Lstat(path); statErr == nil {
				return fmt.Errorf("%w: %s (pass --force to replace it)", ErrOutputExists, path)
			}
			return fmt.Errorf("publish report output %q: %w", path, err)
		}
		if err := os.Remove(temporaryPath); err != nil {
			return fmt.Errorf("remove temporary report after publish: %w", err)
		}
	}

	if err := syncDirectory(directory); err != nil {
		return fmt.Errorf("sync report destination directory %q: %w", directory, err)
	}
	return nil
}
