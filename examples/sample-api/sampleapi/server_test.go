package sampleapi

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func TestDeterministicRoutes(t *testing.T) {
	server := httptest.NewServer(NewHandler())
	t.Cleanup(server.Close)

	tests := []struct {
		path      string
		status    int
		body      string
		redirects bool
	}{
		{path: "/fast", status: http.StatusOK, body: `"route":"fast"`},
		{path: "/delay?ms=0", status: http.StatusOK, body: `"delay_ms":0`},
		{path: "/stream?chunks=3&interval_ms=0", status: http.StatusOK, body: "chunk 3/3"},
		{path: "/error?status=429", status: http.StatusTooManyRequests, body: `"status":429`},
		{path: "/redirect", status: http.StatusFound, body: "/fast", redirects: true},
	}
	for _, test := range tests {
		t.Run(test.path, func(t *testing.T) {
			client := server.Client()
			if test.redirects {
				client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }
			}
			response, err := client.Get(server.URL + test.path)
			if err != nil {
				t.Fatal(err)
			}
			defer response.Body.Close()
			body, _ := io.ReadAll(response.Body)
			if response.StatusCode != test.status || !strings.Contains(string(body), test.body) && response.Header.Get("Location") != test.body {
				t.Fatalf("response = %d %q, location %q", response.StatusCode, body, response.Header.Get("Location"))
			}
		})
	}
}

func TestConcurrencyRouteReportsOverlap(t *testing.T) {
	server := httptest.NewServer(NewHandler())
	t.Cleanup(server.Close)
	start := make(chan struct{})
	values := make(chan string, 2)
	var group sync.WaitGroup
	for range 2 {
		group.Add(1)
		go func() {
			defer group.Done()
			<-start
			response, err := server.Client().Get(server.URL + "/concurrency?delay_ms=25")
			if err != nil {
				t.Error(err)
				return
			}
			defer response.Body.Close()
			values <- response.Header.Get("X-Sloth-In-Flight")
		}()
	}
	close(start)
	group.Wait()
	close(values)
	foundOverlap := false
	for value := range values {
		foundOverlap = foundOverlap || value == "2"
	}
	if !foundOverlap {
		t.Fatal("concurrency route did not observe overlapping requests")
	}
}

func TestRoutesRejectUnboundedParameters(t *testing.T) {
	server := httptest.NewServer(NewHandler())
	t.Cleanup(server.Close)
	for _, path := range []string{"/delay?ms=5001", "/stream?chunks=101", "/error?status=200", "/redirect?to=https://example.com", "/redirect?to=//example.com"} {
		response, err := server.Client().Get(server.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		if response.StatusCode != http.StatusBadRequest {
			t.Fatalf("%s status = %d", path, response.StatusCode)
		}
	}
}
