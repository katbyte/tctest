package chttp

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httputil"
	"strings"

	"github.com/katbyte/tctest/lib/clog"
)

var HTTP = http.DefaultClient

func NewHTTPClient(name string) *http.Client {
	return &http.Client{
		Transport: NewTransport(name, http.DefaultTransport),
	}
}

type transport struct {
	name      string
	transport http.RoundTripper
}

func (t *transport) RoundTrip(req *http.Request) (*http.Response, error) {
	reqData, err := httputil.DumpRequestOut(req, true)

	if err == nil {
		clog.Log.Tracef(logReqMsg, t.name, prettyPrintJSON(reqData))
	} else {
		clog.Log.Debugf("%s API Request error: %#v", t.name, err)
	}

	resp, err := t.transport.RoundTrip(req)
	if err != nil {
		return resp, err
	}

	respData, err := httputil.DumpResponse(resp, true)
	if err == nil {
		clog.Log.Tracef(logRespMsg, t.name, prettyPrintJSON(respData))
	} else {
		clog.Log.Debugf("%s API Response error: %#v", t.name, err)
	}

	return resp, nil
}

func NewTransport(name string, t http.RoundTripper) *transport {
	return &transport{name, t}
}

// prettyPrintJSON iterates through a []byte line-by-line,
// transforming any lines that are complete json into pretty-printed json.
func prettyPrintJSON(b []byte) string {
	parts := strings.Split(string(b), "\n")
	for i, p := range parts {
		if b := []byte(p); json.Valid(b) {
			var out bytes.Buffer
			// nolint:errcheck
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
