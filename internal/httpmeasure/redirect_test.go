package httpmeasure

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestRedirectsAreDisabledByDefault(t *testing.T) {
	finalRequests := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/start" {
			http.Redirect(writer, request, "/final?token=destination-secret", http.StatusFound)
			return
		}
		finalRequests++
		_, _ = io.WriteString(writer, "final")
	}))
	defer server.Close()

	controller := mustRedirectController(t, false, 5)
	client := &http.Client{CheckRedirect: controller.CheckRedirect, Timeout: time.Second}
	response, err := client.Get(server.URL + "/start?token=source-secret")
	if err != nil {
		t.Fatalf("GET redirect: %v", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusFound || finalRequests != 0 {
		t.Fatalf("status/final requests = (%d, %d), want (302, 0)", response.StatusCode, finalRequests)
	}
	result := controller.Result(response.Request.URL)
	if len(result.Hops) != 1 || result.Hops[0].Followed {
		t.Fatalf("redirect result = %#v", result)
	}
	assertRedirectSecretsAbsent(t, result, "source-secret", "destination-secret")
}

func TestRedirectFollowingRecordsHopsAndFinalTarget(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/one":
			http.Redirect(writer, request, "/two?key=middle-secret", http.StatusMovedPermanently)
		case "/two":
			http.Redirect(writer, request, "/three?key=final-secret", http.StatusTemporaryRedirect)
		default:
			_, _ = io.WriteString(writer, "done")
		}
	}))
	defer server.Close()

	controller := mustRedirectController(t, true, 2)
	client := &http.Client{CheckRedirect: controller.CheckRedirect, Timeout: time.Second}
	response, err := client.Get(server.URL + "/one")
	if err != nil {
		t.Fatalf("GET redirect chain: %v", err)
	}
	defer response.Body.Close()

	result := controller.Result(response.Request.URL)
	if len(result.Hops) != 2 || !result.Hops[0].Followed || !result.Hops[1].Followed {
		t.Fatalf("redirect hops = %#v", result.Hops)
	}
	if result.Hops[0].StatusCode != http.StatusMovedPermanently || result.Hops[1].StatusCode != http.StatusTemporaryRedirect {
		t.Fatalf("redirect statuses = (%d, %d)", result.Hops[0].StatusCode, result.Hops[1].StatusCode)
	}
	if result.FinalTarget == nil || result.FinalTarget.Path != "/three" {
		t.Fatalf("final target = %#v", result.FinalTarget)
	}
	assertRedirectSecretsAbsent(t, result, "middle-secret", "final-secret")
}

func TestRedirectLimitIsEnforced(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/one":
			http.Redirect(writer, request, "/two", http.StatusFound)
		case "/two":
			http.Redirect(writer, request, "/three", http.StatusFound)
		default:
			http.Redirect(writer, request, "/four", http.StatusFound)
		}
	}))
	defer server.Close()

	controller := mustRedirectController(t, true, 2)
	client := &http.Client{CheckRedirect: controller.CheckRedirect, Timeout: time.Second}
	response, err := client.Get(server.URL + "/one")
	if response != nil {
		_ = response.Body.Close()
	}
	var limitErr *RedirectLimitError
	if !errors.As(err, &limitErr) || limitErr.Limit != 2 {
		t.Fatalf("GET error = %v, want RedirectLimitError(2)", err)
	}
	result := controller.Result(nil)
	if len(result.Hops) != 3 || result.Hops[2].Followed {
		t.Fatalf("redirect hops = %#v", result.Hops)
	}
}

func TestCrossOriginRedirectStripsSensitiveHeaders(t *testing.T) {
	received := make(chan http.Header, 1)
	destination := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		received <- request.Header.Clone()
		_, _ = io.WriteString(writer, request.Method)
	}))
	defer destination.Close()

	source := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		http.Redirect(writer, request, destination.URL+"/final", http.StatusFound)
	}))
	defer source.Close()

	controller := mustRedirectController(t, true, 5)
	client := &http.Client{CheckRedirect: controller.CheckRedirect, Timeout: time.Second}
	request, err := http.NewRequest(http.MethodPost, source.URL+"/start?token=secret", strings.NewReader("body"))
	if err != nil {
		t.Fatalf("http.NewRequest() error = %v", err)
	}
	request.Header.Set("Authorization", "Bearer secret")
	request.Header.Set("Proxy-Authorization", "Basic secret")
	request.Header.Set("Cookie", "session=secret")
	request.Header.Set("X-API-Token", "secret")
	request.Header.Set("Referer", source.URL+"/?token=secret")

	response, err := client.Do(request)
	if err != nil {
		t.Fatalf("client.Do() error = %v", err)
	}
	defer response.Body.Close()

	headers := <-received
	for _, name := range []string{"Authorization", "Proxy-Authorization", "Cookie", "X-API-Token", "Referer"} {
		if value := headers.Get(name); value != "" {
			t.Fatalf("cross-origin %s = %q, want empty", name, value)
		}
	}
	result := controller.Result(response.Request.URL)
	if len(result.Hops) != 1 || !result.Hops[0].CrossOrigin || result.Hops[0].Method != http.MethodGet {
		t.Fatalf("cross-origin hop = %#v", result.Hops)
	}
}

func TestRedirectResultIsDefensive(t *testing.T) {
	controller := mustRedirectController(t, false, 5)
	source, _ := http.NewRequest(http.MethodGet, "https://example.com/start", nil)
	target, _ := http.NewRequest(http.MethodGet, "https://example.com/end?token=secret", nil)
	target.Response = &http.Response{StatusCode: http.StatusFound}
	_ = controller.CheckRedirect(target, []*http.Request{source})

	first := controller.Result(target.URL)
	first.Hops[0].Target.QueryKeys[0] = "changed"
	first.FinalTarget.QueryKeys[0] = "changed"
	second := controller.Result(target.URL)
	if second.Hops[0].Target.QueryKeys[0] != "token" || second.FinalTarget.QueryKeys[0] != "token" {
		t.Fatalf("result mutation changed controller state: %#v", second)
	}
}

func mustRedirectController(t *testing.T, enabled bool, limit int) *RedirectController {
	t.Helper()
	controller, err := NewRedirectController(enabled, limit)
	if err != nil {
		t.Fatalf("NewRedirectController() error = %v", err)
	}
	return controller
}

func assertRedirectSecretsAbsent(t *testing.T, result RedirectResult, secrets ...string) {
	t.Helper()
	formatted := fmt.Sprintf("%#v", result)
	for _, secret := range secrets {
		if strings.Contains(formatted, secret) {
			t.Fatalf("redirect result contains secret %q: %s", secret, formatted)
		}
	}
}
