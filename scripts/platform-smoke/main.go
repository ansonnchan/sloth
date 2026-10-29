// Command platform-smoke verifies an installed Sloth binary on its native OS.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/ansonnchan/sloth/internal/releasecheck"
)

func main() {
	binary := flag.String("binary", "", "path to the native Sloth binary")
	version := flag.String("version", "", "expected version")
	commit := flag.String("commit", "", "expected commit")
	target := flag.String("target", "", "expected GOOS-GOARCH target")
	flag.Parse()
	if err := releasecheck.RunPlatformSmoke(*binary, *version, *commit, *target); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
