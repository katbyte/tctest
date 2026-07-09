package chttp

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httputil"
	"strings"
	"time"

	"github.com/katbyte/tctest/lib/clog"
	"github.com/sirupsen/logrus"
)

var HTTP = http.DefaultClient

func NewHTTPClient(name string) *http.Client {
	return &http.Client{
		Transport: NewRetryTransport(name, NewTransport(name, http.DefaultTransport), 3),
	}
}

type Transport struct {
	name      string
	transport http.RoundTripper
}

func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	if clog.Log.IsLevelEnabled(logrus.TraceLevel) {
		reqData, err := httputil.DumpRequestOut(req, true)
		if err == nil {
			clog.Log.Tracef(logReqMsg, t.name, prettyPrintJSON(reqData))
		} else {
			clog.Log.Debugf("%s API Request error: %#v", t.name, err)
		}
	}

	resp, err := t.transport.RoundTrip(req)
	if err != nil {
		return resp, err
	}

	if clog.Log.IsLevelEnabled(logrus.TraceLevel) {
		respData, err := httputil.DumpResponse(resp, true)
		if err == nil {
			clog.Log.Tracef(logRespMsg, t.name, prettyPrintJSON(respData))
		} else {
			clog.Log.Debugf("%s API Response error: %#v", t.name, err)
		}
	}

	return resp, nil
}

func NewTransport(name string, t http.RoundTripper) *Transport {
	return &Transport{name, t}
}

// RetryTransport wraps an http.RoundTripper with retry logic for transient failures.
// It retries on connection errors, 429 (rate limited), and 5xx (server error) responses.
type RetryTransport struct {
	name      string
	transport http.RoundTripper
	maxRetry  int
}

func NewRetryTransport(name string, t http.RoundTripper, maxRetry int) *RetryTransport {
	return &RetryTransport{name: name, transport: t, maxRetry: maxRetry}
}

func (t *RetryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	var resp *http.Response
	var err error

	for attempt := range t.maxRetry {
		resp, err = t.transport.RoundTrip(req)

		if err != nil {
			if attempt < t.maxRetry-1 {
				wait := time.Duration(1<<attempt) * time.Second
				clog.Log.Debugf("%s request failed (attempt %d/%d), retrying in %s: %v", t.name, attempt+1, t.maxRetry, wait, err)
				time.Sleep(wait)
				continue
			}
			return nil, err
		}

		// retry on 429 (rate limited) or 5xx (server error)
		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
			if attempt < t.maxRetry-1 {
				wait := time.Duration(1<<attempt) * time.Second
				clog.Log.Debugf("%s got status %d (attempt %d/%d), retrying in %s", t.name, resp.StatusCode, attempt+1, t.maxRetry, wait)
				_ = resp.Body.Close()
				time.Sleep(wait)
				continue
			}
		}

		return resp, nil
	}

	return resp, err
}

// prettyPrintJSON iterates through a []byte line-by-line,
// transforming any lines that are complete json into pretty-printed json.
func prettyPrintJSON(b []byte) string {
	parts := strings.Split(string(b), "\n")
	for i, p := range parts {
		if b := []byte(p); json.Valid(b) {
			var out bytes.Buffer
			//nolint:errcheck,gosec // error is intentionally ignored for pretty printing
			json.Indent(&out, b, "", " ")
			parts[i] = out.String()
		}
	}

	return strings.Join(parts, "\n")
}

const logReqMsg = `%s API Request Details:
---[ REQUEST ]---------------------------------------
%s
-----------------------------------------------------`

const logRespMsg = `%s API Response Details:
---[ RESPONSE ]--------------------------------------
%s
-----------------------------------------------------`
