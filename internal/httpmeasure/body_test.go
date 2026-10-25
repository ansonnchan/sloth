package httpmeasure

import (
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDrainBody(t *testing.T) {
	cases := []struct {
		name      string
		body      string
		limit     int64
		wantBytes int64
		truncated bool
	}{
		{name: "empty", body: "", limit: 10, wantBytes: 0},
		{name: "under limit", body: "sloth", limit: 10, wantBytes: 5},
		{name: "exact limit", body: "sloth", limit: 5, wantBytes: 5},
		{name: "over limit", body: "sloths", limit: 5, wantBytes: 6, truncated: true},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			body := &trackingReadCloser{Reader: strings.NewReader(test.body)}
			result, err := DrainBody(body, test.limit)
			if !body.closed {
				t.Fatal("body was not closed")
			}
			if result.TransferredBytes != test.wantBytes || result.Truncated != test.truncated {
				t.Fatalf("result = %#v, want bytes %d truncated %v", result, test.wantBytes, test.truncated)
			}
			var limitErr *BodyLimitError
			if errors.As(err, &limitErr) != test.truncated {
				t.Fatalf("BodyLimitError present = %v, want %v; error %v", errors.As(err, &limitErr), test.truncated, err)
			}
			if result.Complete == test.truncated || result.ReuseEligible == test.truncated {
				t.Fatalf("completion/reuse eligibility = (%v, %v), truncated %v", result.Complete, result.ReuseEligible, test.truncated)
			}
		})
	}
}

func TestDrainBodyClosesAfterReadAndCloseFailures(t *testing.T) {
	readErr := errors.New("read failed")
	closeErr := errors.New("close failed")
	body := &trackingReadCloser{Reader: &failingReader{err: readErr}, closeErr: closeErr}
	result, err := DrainBody(body, 10)

	if !body.closed || result.Complete || result.ReuseEligible {
		t.Fatalf("result = %#v, closed = %v", result, body.closed)
	}
	if !errors.Is(err, readErr) || !errors.Is(err, closeErr) {
		t.Fatalf("error = %v, want both read and close causes", err)
	}
}

func TestDrainBodyUsesOnlyOneProbeBytePastLimit(t *testing.T) {
	reader := &countingReadCloser{remaining: 1 << 30}
	result, err := DrainBody(reader, 1024)
	var limitErr *BodyLimitError
	if !errors.As(err, &limitErr) {
		t.Fatalf("DrainBody() error = %v, want BodyLimitError", err)
	}
	if reader.read != 1025 || result.TransferredBytes != 1025 {
		t.Fatalf("read bytes = (%d, %d), want 1025", reader.read, result.TransferredBytes)
	}
}

func TestDrainBodyWithChunkedStreamingAndCompressedResponses(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/chunked":
			flusher := writer.(http.Flusher)
			_, _ = io.WriteString(writer, "slow")
			flusher.Flush()
			_, _ = io.WriteString(writer, " body")
		case "/compressed":
			writer.Header().Set("Content-Encoding", "gzip")
			compressed := gzip.NewWriter(writer)
			_, _ = io.WriteString(compressed, "decoded body")
			_ = compressed.Close()
		default:
			writer.WriteHeader(http.StatusNoContent)
		}
	}))
	defer server.Close()

	for _, test := range []struct {
		path      string
		wantBytes int64
	}{
		{path: "/empty", wantBytes: 0},
		{path: "/chunked", wantBytes: int64(len("slow body"))},
		{path: "/compressed", wantBytes: int64(len("decoded body"))},
	} {
		response, err := http.Get(server.URL + test.path)
		if err != nil {
			t.Fatalf("GET %s: %v", test.path, err)
		}
		result, err := DrainBody(response.Body, 1024)
		if err != nil {
			t.Fatalf("DrainBody(%s) error = %v", test.path, err)
		}
		if result.TransferredBytes != test.wantBytes || !result.Complete {
			t.Fatalf("DrainBody(%s) = %#v, want %d complete bytes", test.path, result, test.wantBytes)
		}
	}
}

func TestDrainBodyReportsPartialServerResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		hijacker := writer.(http.Hijacker)
		connection, buffer, err := hijacker.Hijack()
		if err != nil {
			t.Errorf("Hijack() error = %v", err)
			return
		}
		_, _ = fmt.Fprint(buffer, "HTTP/1.1 200 OK\r\nContent-Length: 10\r\n\r\nshort")
		_ = buffer.Flush()
		_ = connection.Close()
	}))
	defer server.Close()

	response, err := http.Get(server.URL)
	if err != nil {
		t.Fatalf("GET partial server: %v", err)
	}
	result, err := DrainBody(response.Body, 1024)
	if !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("DrainBody() error = %v, want unexpected EOF", err)
	}
	if result.TransferredBytes != int64(len("short")) || result.Complete || result.ReuseEligible {
		t.Fatalf("partial result = %#v", result)
	}
}

type trackingReadCloser struct {
	io.Reader
	closed   bool
	closeErr error
}

func (r *trackingReadCloser) Close() error {
	r.closed = true
	return r.closeErr
}

type failingReader struct{ err error }

func (r *failingReader) Read([]byte) (int, error) { return 0, r.err }

type countingReadCloser struct {
	remaining int64
	read      int64
}

func (r *countingReadCloser) Read(buffer []byte) (int, error) {
	if r.remaining == 0 {
		return 0, io.EOF
	}
	count := int64(len(buffer))
	if count > r.remaining {
		count = r.remaining
	}
	r.remaining -= count
	r.read += count
	return int(count), nil
}

func (r *countingReadCloser) Close() error { return nil }
