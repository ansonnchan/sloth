// Package sampleapi provides deterministic local HTTP behavior for examples
// and end-to-end tests. It is not production server infrastructure.
package sampleapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"sync/atomic"
	"time"
)

const (
	maxDelay       = 5 * time.Second
	maxStreamParts = 100
)

// API owns route-local state, including the concurrency observation counter.
type API struct {
	inFlight atomic.Int64
}

// NewHandler returns a fresh deterministic sample API.
func NewHandler() http.Handler {
	api := &API{}
	mux := http.NewServeMux()
	mux.HandleFunc("/", api.index)
	mux.HandleFunc("/healthz", api.health)
	mux.HandleFunc("/fast", api.fast)
	mux.HandleFunc("/delay", api.delay)
	mux.HandleFunc("/stream", api.stream)
	mux.HandleFunc("/error", api.errorResponse)
	mux.HandleFunc("/concurrency", api.concurrency)
	mux.HandleFunc("/redirect", api.redirect)
	return mux
}

func (api *API) index(writer http.ResponseWriter, request *http.Request) {
	if request.URL.Path != "/" {
		http.NotFound(writer, request)
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{
		"name":   "sloth sample API",
		"routes": []string{"/fast", "/delay?ms=100", "/stream?chunks=5&interval_ms=50", "/error?status=503", "/concurrency?delay_ms=25", "/redirect"},
	})
}

func (api *API) health(writer http.ResponseWriter, _ *http.Request) {
	writeJSON(writer, http.StatusOK, map[string]string{"status": "ok"})
}

func (api *API) fast(writer http.ResponseWriter, _ *http.Request) {
	writeJSON(writer, http.StatusOK, map[string]string{"route": "fast"})
}

func (api *API) delay(writer http.ResponseWriter, request *http.Request) {
	delay, err := durationParameter(request, "ms", 100)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}
	time.Sleep(delay)
	writeJSON(writer, http.StatusOK, map[string]any{"route": "delay", "delay_ms": delay.Milliseconds()})
}

func (api *API) stream(writer http.ResponseWriter, request *http.Request) {
	chunks, err := integerParameter(request, "chunks", 5, 1, maxStreamParts)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}
	interval, err := durationParameter(request, "interval_ms", 50)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}
	flusher, ok := writer.(http.Flusher)
	if !ok {
		http.Error(writer, "streaming is unavailable", http.StatusInternalServerError)
		return
	}
	writer.Header().Set("Content-Type", "text/plain; charset=utf-8")
	writer.WriteHeader(http.StatusOK)
	for index := range chunks {
		_, _ = fmt.Fprintf(writer, "chunk %d/%d\n", index+1, chunks)
		flusher.Flush()
		if index+1 < chunks {
			time.Sleep(interval)
		}
	}
}

func (api *API) errorResponse(writer http.ResponseWriter, request *http.Request) {
	status, err := integerParameter(request, "status", http.StatusServiceUnavailable, 400, 599)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(writer, status, map[string]any{"route": "error", "status": status})
}

func (api *API) concurrency(writer http.ResponseWriter, request *http.Request) {
	baseDelay, err := durationParameter(request, "delay_ms", 25)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}
	inFlight := api.inFlight.Add(1)
	defer api.inFlight.Add(-1)
	delay := time.Duration(inFlight) * baseDelay
	if delay > maxDelay {
		delay = maxDelay
	}
	writer.Header().Set("X-Sloth-In-Flight", strconv.FormatInt(inFlight, 10))
	time.Sleep(delay)
	writeJSON(writer, http.StatusOK, map[string]any{
		"route": "concurrency", "in_flight": inFlight, "delay_ms": delay.Milliseconds(),
	})
}

func (api *API) redirect(writer http.ResponseWriter, request *http.Request) {
	target := request.URL.Query().Get("to")
	if target == "" {
		target = "/fast"
	}
	if target[0] != '/' || len(target) > 256 || len(target) > 1 && target[1] == '/' {
		http.Error(writer, "to must be a local absolute path", http.StatusBadRequest)
		return
	}
	http.Redirect(writer, request, target, http.StatusFound)
}

func durationParameter(request *http.Request, name string, fallback int) (time.Duration, error) {
	milliseconds, err := integerParameter(request, name, fallback, 0, int(maxDelay/time.Millisecond))
	return time.Duration(milliseconds) * time.Millisecond, err
}

func integerParameter(request *http.Request, name string, fallback, minimum, maximum int) (int, error) {
	raw := request.URL.Query().Get(name)
	if raw == "" {
		return fallback, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < minimum || value > maximum {
		return 0, fmt.Errorf("%s must be an integer from %d to %d", name, minimum, maximum)
	}
	return value, nil
}

func writeJSON(writer http.ResponseWriter, status int, value any) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(value)
}
