package hf

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// SearchItem is a single result from a Hugging Face model search.
type SearchItem struct {
	ID          string   `json:"id"`
	PipelineTag string   `json:"pipeline_tag,omitempty"`
	Downloads   int64    `json:"downloads,omitempty"`
	Likes       int64    `json:"likes,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	LibraryName string   `json:"library_name,omitempty"`
	Gated       bool     `json:"gated,omitempty"`
	Private     bool     `json:"private,omitempty"`
}

// hubSearchModel mirrors the relevant fields of the HF /api/models list items.
type hubSearchModel struct {
	ID          string   `json:"id"`
	ModelID     string   `json:"modelId"`
	PipelineTag string   `json:"pipeline_tag"`
	Downloads   int64    `json:"downloads"`
	Likes       int64    `json:"likes"`
	Tags        []string `json:"tags"`
	LibraryName string   `json:"library_name"`
	Gated       any      `json:"gated"`
	Private     bool     `json:"private"`
}

// Search queries the Hugging Face Hub model index, sorted by downloads.
func (c *Client) Search(ctx context.Context, query, task string, limit int) ([]SearchItem, error) {
	if limit <= 0 || limit > 50 {
		limit = 16
	}
	q := url.Values{}
	q.Set("search", strings.TrimSpace(query))
	q.Set("sort", "downloads")
	q.Set("direction", "-1")
	q.Set("limit", fmt.Sprintf("%d", limit))
	if task != "" && task != "any" {
		q.Set("pipeline_tag", task)
	}
	u := c.BaseURL + "/api/models?" + q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	applyAuth(req, c.Token)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("hugging face search failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("hugging face search returned %d", resp.StatusCode)
	}

	var raw []hubSearchModel
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("decode hf search: %w", err)
	}
	out := make([]SearchItem, 0, len(raw))
	for _, m := range raw {
		id := m.ID
		if id == "" {
			id = m.ModelID
		}
		out = append(out, SearchItem{
			ID:          id,
			PipelineTag: m.PipelineTag,
			Downloads:   m.Downloads,
			Likes:       m.Likes,
			Tags:        m.Tags,
			LibraryName: m.LibraryName,
			Gated:       gatedTrue(m.Gated),
			Private:     m.Private,
		})
	}
	return out, nil
}
