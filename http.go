package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/url"
	"slices"
	"strings"
)

var httpClient = &http.Client{Timeout: timeout, CheckRedirect: checkRedirect}

// checkRedirect follows a redirect only if it keeps the method. On a 301,
// 302 or 303, net/http would follow a POST with a GET that has no body,
// which loses the message but still gets a 2xx from most servers.
func checkRedirect(req *http.Request, via []*http.Request) error {
	if req.Method != via[0].Method {
		return fmt.Errorf("redirected with HTTP %s, which would drop the message; use the URL it redirects to", req.Response.Status)
	}
	if len(via) >= 10 {
		return errors.New("stopped after 10 redirects")
	}
	return nil
}

// redacted stands in for secrets in --dry-run output.
const redacted = "REDACTED"

// request is an HTTP request a sink wants to make, kept as plain data so
// that --dry-run can print it instead.
type request struct {
	method string
	url    string
	header http.Header
	body   []byte
}

// jsonRequest builds a POST of v as JSON.
func jsonRequest(endpoint string, v any) (*request, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return &request{
		method: http.MethodPost,
		url:    endpoint,
		header: http.Header{"Content-Type": {"application/json"}},
		body:   bytes.TrimSuffix(buf.Bytes(), []byte("\n")),
	}, nil
}

// formRequest builds a URL-encoded form POST.
func formRequest(endpoint string, form url.Values) *request {
	return &request{
		method: http.MethodPost,
		url:    endpoint,
		header: http.Header{"Content-Type": {"application/x-www-form-urlencoded"}},
		body:   []byte(form.Encode()),
	}
}

// String formats r for --dry-run, much as it would go over the wire.
func (r *request) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s %s\n", r.method, r.url)
	for _, k := range slices.Sorted(maps.Keys(r.header)) {
		for _, v := range r.header[k] {
			fmt.Fprintf(&b, "%s: %s\n", k, v)
		}
	}
	fmt.Fprintf(&b, "\n%s\n", r.body)
	return b.String()
}

// send makes the request and turns a non-2xx response into an error.
func (r *request) send(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, r.method, r.url, bytes.NewReader(r.body))
	if err != nil {
		return scrubURL(err)
	}
	for k, vs := range r.header {
		req.Header[k] = vs
	}
	req.Header.Set("User-Agent", "notify/"+versionString())
	resp, err := httpClient.Do(req)
	if err != nil {
		return scrubURL(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return &httpError{Status: resp.Status, Detail: errorDetail(b)}
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	return nil
}

// sendRequest and previewRequest implement Sink.Send and Sink.Preview for
// sinks that make a single HTTP request.
func sendRequest(ctx context.Context, build func(Message) (*request, error), m Message) error {
	r, err := build(m)
	if err != nil {
		return err
	}
	return r.send(ctx)
}

func previewRequest(build func(Message) (*request, error), m Message) (string, error) {
	r, err := build(m)
	if err != nil {
		return "", err
	}
	return r.String(), nil
}

// scrubURL redacts the URL that net/http puts in its errors. For a webhook
// the URL is the secret, and the error ends up in scrollback and CI logs.
func scrubURL(err error) error {
	var ue *url.Error
	if !errors.As(err, &ue) {
		return err
	}
	if ue.Timeout() {
		return fmt.Errorf("%s %s: timed out after %v", ue.Op, redactURL(ue.URL), timeout)
	}
	return fmt.Errorf("%s %s: %w", ue.Op, redactURL(ue.URL), ue.Err)
}

// redactURL keeps only the scheme and host of a webhook URL, since the
// rest of it is the secret.
func redactURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return redacted
	}
	return u.Scheme + "://" + u.Host + "/" + redacted
}

// httpError is a non-2xx response from a service.
type httpError struct {
	Status string // e.g. "404 Not Found"
	Detail string // reason given in the response body, if any
}

func (e *httpError) Error() string {
	if e.Detail == "" {
		return "HTTP " + e.Status
	}
	return fmt.Sprintf("HTTP %s: %s", e.Status, e.Detail)
}

// errorDetail pulls a short, single-line reason out of an error response.
// JSON bodies from Pushover ("errors"), Discord ("message") and ntfy
// ("error") are reduced to their reason; anything else is used as is.
func errorDetail(body []byte) string {
	var v struct {
		Errors  []string `json:"errors"`
		Message string   `json:"message"`
		Error   string   `json:"error"`
	}
	// Unmarshal fills what it can even when other fields don't match.
	_ = json.Unmarshal(body, &v)
	switch {
	case len(v.Errors) > 0:
		return strings.Join(v.Errors, "; ")
	case v.Message != "":
		return v.Message
	case v.Error != "":
		return v.Error
	}
	return truncate(strings.Join(strings.Fields(string(body)), " "), 200)
}
