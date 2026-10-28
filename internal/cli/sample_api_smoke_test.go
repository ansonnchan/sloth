package cli

import (
	"bytes"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/ansonnchan/sloth/examples/sample-api/sampleapi"
	"github.com/ansonnchan/sloth/internal/buildinfo"
)

func TestSampleAPISmokeWorkflows(t *testing.T) {
	server := httptest.NewServer(sampleapi.NewHandler())
	t.Cleanup(server.Close)

	run := func(arguments ...string) {
		t.Helper()
		var stdout bytes.Buffer
		var stderr bytes.Buffer
		if code := Run(arguments, &stdout, &stderr, buildinfo.Info{Version: "test"}); code != int(ExitSuccess) {
			t.Fatalf("sloth %v = %d\nstdout: %s\nstderr: %s", arguments, code, stdout.String(), stderr.String())
		}
	}

	run("inspect", "--json", server.URL+"/fast")
	run("diagnose", "--json", "--requests", "10", "--warmups", "0", "--rate", "100", "--concurrency", "1,2", server.URL+"/concurrency?delay_ms=0")

	before := filepath.Join(t.TempDir(), "before.json")
	after := filepath.Join(t.TempDir(), "after.json")
	run("export", "--quiet", "--output", before, "--requests", "10", "--warmups", "0", "--rate", "100", server.URL+"/delay?ms=1")
	run("export", "--quiet", "--output", after, "--requests", "10", "--warmups", "0", "--rate", "100", server.URL+"/delay?ms=1")
	run("compare", before, after)
}
