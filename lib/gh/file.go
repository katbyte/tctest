package gh

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// DownloadFile fetches the raw content of a file from GitHub at a specific ref (commit SHA or branch).
// Returns the file content bytes, the HTTP status code, and any error.
func (r Repo) DownloadFile(ctx context.Context, httpClient *http.Client, path, ref string) (content []byte, statusCode int, err error) {
	base := "https://raw.githubusercontent.com"
	if r.RawURL != "" {
		base = strings.TrimSuffix(r.RawURL, "/")
	}
	rawURL := fmt.Sprintf("%s/%s/%s/%s/%s", base, r.Owner, r.Name, ref, path)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, http.NoBody)
	if err != nil {
		return nil, 0, fmt.Errorf("creating request for %s: %w", path, err)
	}

	if r.Token.Token != nil {
		req.Header.Set("Authorization", "token "+*r.Token.Token)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("downloading file (%s): %w", path, err)
	}

	content, err = io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("reading file (%s): %w", path, err)
	}

	return content, resp.StatusCode, nil
}
