package httpmeasure

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestRequestSpecSeparatesRuntimeDataFromMetadata(t *testing.T) {
	const (
		querySecret   = "query-secret"
		authorization = "Bearer header-secret"
		customSecret  = "custom-header-secret"
	)

	spec, err := NewRequestSpec(RequestSpecInput{
		Target: "https://Example.COM:443/v1/items?token=" + querySecret + "&page=2",
		Method: "get",
		Headers: http.Header{
			"Authorization":   []string{authorization},
			"X-Account-Token": []string{customSecret},
			"Accept":          []string{"application/json"},
		},
		Options: RequestOptions{
			Timeout:       3 * time.Second,
			RedirectLimit: 5,
			BodyLimit:     1024,
		},
	})
	if err != nil {
		t.Fatalf("NewRequestSpec() error = %v", err)
	}

	if got := spec.URL().Query().Get("token"); got != querySecret {
		t.Fatalf("runtime URL token = %q, want original secret", got)
	}
	if got := spec.Headers().Get("Authorization"); got != authorization {
		t.Fatalf("runtime authorization = %q, want original secret", got)
	}

	metadataJSON, err := json.Marshal(spec.Metadata())
	if err != nil {
		t.Fatalf("json.Marshal(metadata) error = %v", err)
	}
	formatted := strings.Join([]string{
		spec.String(),
		fmt.Sprintf("%v", spec),
		fmt.Sprintf("%+v", spec),
		fmt.Sprintf("%#v", spec),
		string(metadataJSON),
	}, "\n")
	for _, secret := range []string{querySecret, authorization, customSecret} {
		if strings.Contains(formatted, secret) {
			t.Fatalf("safe formatted values contain secret %q: %s", secret, formatted)
		}
	}
	if !strings.Contains(formatted, redactedValue) {
		t.Fatalf("safe formatted values do not mark redaction: %s", formatted)
	}
	if got := spec.Metadata().Headers.Get("Accept"); got != "application/json" {
		t.Fatalf("safe Accept header = %q", got)
	}
}

func TestEquivalentTargetsShareIdentity(t *testing.T) {
	first := mustRequestSpec(t, "https://EXAMPLE.com:443/path?token=one&page=1")
	second := mustRequestSpec(t, "https://example.com/path?page=999&token=two")

	if first.Identity().RedactedURL != second.Identity().RedactedURL {
		t.Fatalf("redacted URLs differ:\n%s\n%s", first.Identity().RedactedURL, second.Identity().RedactedURL)
	}
	if first.Identity().Fingerprint != second.Identity().Fingerprint {
		t.Fatalf("fingerprints differ: %s != %s", first.Identity().Fingerprint, second.Identity().Fingerprint)
	}
	if strings.Contains(first.Identity().RedactedURL, "one") || strings.Contains(second.Identity().RedactedURL, "999") {
		t.Fatal("redacted URL contains a query value")
	}
}

func TestIdentityDistinguishesQueryShape(t *testing.T) {
	one := mustRequestSpec(t, "https://example.com/path?tag=one")
	two := mustRequestSpec(t, "https://example.com/path?tag=one&tag=two")

	if one.Identity().Fingerprint == two.Identity().Fingerprint {
		t.Fatal("fingerprint does not distinguish repeated query-key shape")
	}
}

func TestRequestSpecReturnsSecretSafeErrors(t *testing.T) {
	const secret = "do-not-print"
	cases := []string{
		"https://user:" + secret + "@example.com/path",
		"https://example.com/%zz?token=" + secret,
	}

	for _, target := range cases {
		_, err := NewRequestSpec(RequestSpecInput{Target: target, Method: http.MethodGet})
		if err == nil {
			t.Fatalf("NewRequestSpec(%q) unexpectedly succeeded", target)
		}
		if strings.Contains(err.Error(), secret) {
			t.Fatalf("error contains secret: %v", err)
		}
	}
}

func TestReturnedRequestValuesCannotMutateSpec(t *testing.T) {
	spec, err := NewRequestSpec(RequestSpecInput{
		Target:  "https://example.com/path?token=secret",
		Method:  http.MethodGet,
		Headers: http.Header{"Accept": []string{"application/json"}},
	})
	if err != nil {
		t.Fatalf("NewRequestSpec() error = %v", err)
	}

	spec.URL().Host = "attacker.example"
	spec.Headers().Set("Accept", "text/plain")
	identity := spec.Identity()
	identity.QueryKeys[0] = "changed"

	if got := spec.URL().Host; got != "example.com" {
		t.Fatalf("spec URL host mutated to %q", got)
	}
	if got := spec.Headers().Get("Accept"); got != "application/json" {
		t.Fatalf("spec header mutated to %q", got)
	}
	if got := spec.Identity().QueryKeys[0]; got != "token" {
		t.Fatalf("identity query key mutated to %q", got)
	}
}

func mustRequestSpec(t *testing.T, target string) RequestSpec {
	t.Helper()
	spec, err := NewRequestSpec(RequestSpecInput{Target: target, Method: http.MethodGet})
	if err != nil {
		t.Fatalf("NewRequestSpec(%q) error = %v", target, err)
	}
	return spec
}
