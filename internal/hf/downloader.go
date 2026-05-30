package hf

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// Downloader fetches model files from the Hugging Face Hub. For the MVP the
// downloader supports fetching individual small files (config.json, tokenizer
// files); full weight downloads are staged behind the model.pull job and are
// intentionally limited.
type Downloader struct {
	BaseURL string
	Token   string
	HTTP    *http.Client
}

// NewDownloader builds a Downloader.
func NewDownloader(token string) *Downloader {
	return &Downloader{
		BaseURL: "https://huggingface.co",
		Token:   token,
		HTTP:    &http.Client{Timeout: 5 * time.Minute},
	}
}

// DownloadFile fetches a single file at the given repo/revision path into
// destDir, returning the local path.
func (d *Downloader) DownloadFile(ctx context.Context, ref Ref, file, destDir string) (string, error) {
	u := fmt.Sprintf("%s/%s/resolve/%s/%s", d.BaseURL, ref.RepoID(), ref.Revision, file)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return "", err
	}
	applyAuth(req, d.Token)

	resp, err := d.HTTP.Do(req)
	if err != nil {
		return "", fmt.Errorf("download %s failed: %w", file, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download %s: status %d", file, resp.StatusCode)
	}

	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return "", err
	}
	dst := filepath.Join(destDir, filepath.Base(file))
	f, err := os.Create(dst)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()
	if _, err := io.Copy(f, resp.Body); err != nil {
		return "", err
	}
	return dst, nil
}
