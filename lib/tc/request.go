package tc

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/katbyte/tctest/lib/common"
)

func (s Server) makeGetRequest(endpoint string) (int, string, error) {
	uri := fmt.Sprintf("https://%s%s", s.Server, endpoint)

	req, err := http.NewRequestWithContext(context.Background(), "GET", uri, nil)
	if err != nil {
		return 0, "", fmt.Errorf("building http request for url %s failed: %w", uri, err)
	}

	return s.performRequest(req)
}

func (s Server) makePostRequest(endpoint, body string) (int, string, error) {
	uri := fmt.Sprintf("https://%s%s", s.Server, endpoint)
	req, err := http.NewRequestWithContext(context.Background(), "POST", uri, strings.NewReader(body))
	if err != nil {
		return 0, "", fmt.Errorf("building http request for url %s failed: %w", uri, err)
	}

	return s.performRequest(req)
}

func (s Server) performRequest(req *http.Request) (int, string, error) {
	if s.token != nil {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", *s.token))
	} else {
		req.SetBasicAuth(*s.User, *s.Pass)
	}

	req.Header.Set("Content-Type", "application/xml")

	resp, err := common.HTTP.Do(req)
	if err != nil {
		return 0, "", fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	// The calling function will figure out what to do with these
	// because e.g. sometimes a 404 is an error, but sometimes it just means something might be queued
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, "", fmt.Errorf("error reading response body: %w", err)
	}

	return resp.StatusCode, string(b), nil
}
