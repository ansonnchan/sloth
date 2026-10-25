package httpmeasure

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
)

// RedirectHop describes one redirect response and the proposed next request
// without retaining raw query values or headers.
type RedirectHop struct {
	Index       int            `json:"index"`
	StatusCode  int            `json:"status_code"`
	Method      string         `json:"method"`
	Source      TargetIdentity `json:"source"`
	Target      TargetIdentity `json:"target"`
	CrossOrigin bool           `json:"cross_origin"`
	Followed    bool           `json:"followed"`
}

// RedirectResult is the safe redirect portion of an inspect result.
type RedirectResult struct {
	Hops        []RedirectHop   `json:"hops"`
	FinalTarget *TargetIdentity `json:"final_target,omitempty"`
}

// RedirectLimitError indicates that following another hop would exceed policy.
type RedirectLimitError struct{ Limit int }

func (e *RedirectLimitError) Error() string {
	return fmt.Sprintf("redirect limit of %d hops exceeded", e.Limit)
}

// RedirectController implements http.Client redirect policy and records safe
// hop metadata.
type RedirectController struct {
	mu      sync.Mutex
	enabled bool
	limit   int
	hops    []RedirectHop
}

// NewRedirectController creates a disabled-by-default or bounded opt-in policy.
func NewRedirectController(enabled bool, limit int) (*RedirectController, error) {
	if limit <= 0 {
		return nil, errors.New("create redirect policy: hop limit must be greater than zero")
	}
	return &RedirectController{enabled: enabled, limit: limit}, nil
}

// CheckRedirect is suitable for http.Client.CheckRedirect.
func (c *RedirectController) CheckRedirect(request *http.Request, via []*http.Request) error {
	if request == nil || request.URL == nil || len(via) == 0 || via[len(via)-1].URL == nil {
		return errors.New("evaluate redirect: incomplete request chain")
	}

	sourceURL := via[len(via)-1].URL
	crossOrigin := !sameOrigin(sourceURL, request.URL)
	if crossOrigin {
		stripSensitiveRedirectHeaders(request.Header)
	}

	statusCode := 0
	if request.Response != nil {
		statusCode = request.Response.StatusCode
	}
	followed := c.enabled && len(via) <= c.limit
	hop := RedirectHop{
		Index:       len(via),
		StatusCode:  statusCode,
		Method:      request.Method,
		Source:      newTargetIdentity(sourceURL),
		Target:      newTargetIdentity(request.URL),
		CrossOrigin: crossOrigin,
		Followed:    followed,
	}

	c.mu.Lock()
	c.hops = append(c.hops, hop)
	c.mu.Unlock()

	if !c.enabled {
		return http.ErrUseLastResponse
	}
	if len(via) > c.limit {
		return &RedirectLimitError{Limit: c.limit}
	}
	return nil
}

// Result snapshots recorded hops and the final response target.
func (c *RedirectController) Result(finalURL *url.URL) RedirectResult {
	c.mu.Lock()
	defer c.mu.Unlock()

	result := RedirectResult{Hops: make([]RedirectHop, len(c.hops))}
	for index, hop := range c.hops {
		result.Hops[index] = cloneRedirectHop(hop)
	}
	if finalURL != nil {
		identity := newTargetIdentity(finalURL)
		result.FinalTarget = &identity
	}
	return result
}

func sameOrigin(first, second *url.URL) bool {
	return strings.EqualFold(first.Scheme, second.Scheme) &&
		strings.EqualFold(first.Hostname(), second.Hostname()) &&
		effectivePort(first) == effectivePort(second)
}

func effectivePort(target *url.URL) string {
	if port := target.Port(); port != "" {
		return port
	}
	if strings.EqualFold(target.Scheme, "http") {
		return "80"
	}
	if strings.EqualFold(target.Scheme, "https") {
		return "443"
	}
	return ""
}

func stripSensitiveRedirectHeaders(headers http.Header) {
	for name := range headers {
		if isSensitiveHeader(name, nil) || strings.EqualFold(name, "Referer") {
			headers.Del(name)
		}
	}
}

func cloneRedirectHop(hop RedirectHop) RedirectHop {
	hop.Source.QueryKeys = append([]string(nil), hop.Source.QueryKeys...)
	hop.Target.QueryKeys = append([]string(nil), hop.Target.QueryKeys...)
	return hop
}
