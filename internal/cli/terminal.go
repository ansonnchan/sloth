package cli

import (
	"io"
	"os"
	"strconv"

	"github.com/ansonnchan/sloth/internal/config"
)

type presentationOptions struct {
	Color   string
	NoColor bool
	ASCII   bool
}

func defaultPresentationOptions() presentationOptions {
	return presentationOptions{Color: string(config.ColorAuto)}
}

func (options presentationOptions) colorMode() config.ColorMode {
	if options.NoColor {
		return config.ColorNever
	}
	return config.ColorMode(options.Color)
}

func terminalWidth() int {
	width, err := strconv.Atoi(os.Getenv("COLUMNS"))
	if err != nil || width < 20 {
		return 80
	}
	return width
}

func useColor(mode config.ColorMode, writer io.Writer) bool {
	// NO_COLOR is an unconditional environment-level safety switch. The
	// explicit --no-color flag is resolved to ColorNever before this function.
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	switch mode {
	case config.ColorAlways:
		return true
	case config.ColorNever:
		return false
	}
	file, ok := writer.(*os.File)
	if !ok {
		return false
	}
	info, err := file.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}
