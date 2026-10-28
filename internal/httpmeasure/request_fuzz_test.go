package httpmeasure

import (
	"net/http"
	"testing"
	"time"
)

func FuzzNewRequestSpec(f *testing.F) {
	f.Add("https://example.com/path?token=secret", "Authorization", "Bearer secret")
	f.Add("http://127.0.0.1:8080/", "X-Test", "value")
	f.Add("%", "Cookie", "session=secret")
	f.Fuzz(func(t *testing.T, target, headerName, headerValue string) {
		if len(target) > 16<<10 || len(headerName) > 4<<10 || len(headerValue) > 16<<10 {
			t.Skip()
		}
		headers := make(http.Header)
		if headerName != "" {
			headers[headerName] = []string{headerValue}
		}
		spec, err := NewRequestSpec(RequestSpecInput{
			Target: target, Method: http.MethodGet, Headers: headers, SecretHeaderNames: []string{headerName},
			Options: RequestOptions{Timeout: time.Second, RedirectLimit: 5, BodyLimit: 1 << 20},
		})
		if err != nil {
			return
		}
		_ = spec.String()
		_ = spec.Metadata()
		_ = spec.Identity()
	})
}
