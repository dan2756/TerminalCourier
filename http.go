package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

var httpClient = &http.Client{Timeout: timeout}

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

// postJSON POSTs v as JSON to url with any extra headers.
func postJSON(ctx context.Context, url string, header http.Header, v any) error {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, &buf)
	if err != nil {
		return err
	}
	for k, vs := range header {
		req.Header[k] = vs
	}
	req.Header.Set("Content-Type", "application/json")
	return do(req)
}

func do(req *http.Request) error {
	req.Header.Set("User-Agent", "notify/"+versionString())
	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return &httpError{Status: resp.Status, Detail: errorDetail(b)}
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	return nil
}

// errorDetail pulls a short, single-line reason out of an error response.
// JSON bodies from Discord ("message") and ntfy ("error") are reduced to
// their reason; anything else is used as is.
func errorDetail(body []byte) string {
	var v struct {
		Message string `json:"message"`
		Error   string `json:"error"`
	}
	// Unmarshal fills what it can even when other fields don't match.
	_ = json.Unmarshal(body, &v)
	switch {
	case v.Message != "":
		return v.Message
	case v.Error != "":
		return v.Error
	}
	s := strings.Join(strings.Fields(string(body)), " ")
	if len(s) > 200 {
		s = s[:200] + "..."
	}
	return s
}
