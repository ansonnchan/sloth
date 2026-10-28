package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ansonnchan/sloth/internal/buildinfo"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func TestCommandReferenceCoversPublicCommandsAndFlags(t *testing.T) {
	path := filepath.Join("..", "..", "docs", "commands.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	document := string(data)
	root := newRootCommand(&bytes.Buffer{}, &bytes.Buffer{}, buildinfo.Info{Version: "test"})
	root.InitDefaultHelpFlag()
	root.InitDefaultVersionFlag()
	root.InitDefaultHelpCmd()
	root.InitDefaultCompletionCmd()
	assertDocumentedFlags(t, document, "sloth", root.LocalNonPersistentFlags())
	assertDocumentedFlags(t, document, "sloth global", root.PersistentFlags())

	walkDocumentedCommands(t, document, path, root)
}

func walkDocumentedCommands(t *testing.T, document, path string, parent interface{ Commands() []*cobra.Command }) {
	t.Helper()
	for _, command := range parent.Commands() {
		command.InitDefaultHelpFlag()
		if !strings.Contains(document, command.Name()) {
			t.Errorf("%s does not document command %q", path, command.CommandPath())
		}
		assertDocumentedFlags(t, document, command.CommandPath(), command.LocalNonPersistentFlags())
		walkDocumentedCommands(t, document, path, command)
	}
}

func assertDocumentedFlags(t *testing.T, document, command string, flags *pflag.FlagSet) {
	t.Helper()
	flags.VisitAll(func(flag *pflag.Flag) {
		if !strings.Contains(document, "--"+flag.Name) {
			t.Errorf("command reference does not document %s flag --%s", command, flag.Name)
		}
	})
}
