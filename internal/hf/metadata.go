package hf

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Client queries the Hugging Face Hub for model metadata.
type Client struct {
	BaseURL string
	Token   string
	HTTP    *http.Client
}

// NewClient builds a Client. token may be empty for public models.
func NewClient(token string) *Client {
	return &Client{
		BaseURL: "https://huggingface.co",
		Token:   token,
		HTTP:    &http.Client{Timeout: 30 * time.Second},
	}
}

// hubModelInfo mirrors the relevant fields of the HF /api/models response.
type hubModelInfo struct {
	ID          string   `json:"id"`
	ModelID     string   `json:"modelId"`
	PipelineTag string   `json:"pipeline_tag"`
	LibraryName string   `json:"library_name"`
	Tags        []string `json:"tags"`
	Gated       any      `json:"gated"`
	Private     bool     `json:"private"`
	CardData    struct {
		License   any `json:"license"`
		BaseModel any `json:"base_model"`
	} `json:"cardData"`
	Config struct {
		ModelType string `json:"model_type"`
	} `json:"config"`
	Siblings []struct {
		Rfilename string `json:"rfilename"`
	} `json:"siblings"`
	SafeTensors struct {
		Total int64 `json:"total"`
	} `json:"safetensors"`
}

// Inspect resolves a model reference into Metadata. It contacts the Hugging
// Face Hub API; a clear error is returned when the network is unavailable or
// the model is gated/private without a token.
func (c *Client) Inspect(ctx context.Context, ref Ref) (*Metadata, error) {
	u := fmt.Sprintf("%s/api/models/%s/revision/%s", c.BaseURL, ref.RepoID(), ref.Revision)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("build hf request: %w", err)
	}
	applyAuth(req, c.Token)

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("hugging face request failed (no network or DNS?): %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	switch resp.StatusCode {
	case http.StatusOK:
		// fall through
	case http.StatusUnauthorized, http.StatusForbidden:
		return nil, fmt.Errorf("model %q is gated or private; set HF_TOKEN with access (status %d)", ref.RepoID(), resp.StatusCode)
	case http.StatusNotFound:
		return nil, fmt.Errorf("model %q not found at revision %q", ref.RepoID(), ref.Revision)
	default:
		return nil, fmt.Errorf("hugging face returned status %d for %q", resp.StatusCode, ref.RepoID())
	}

	var info hubModelInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, fmt.Errorf("decode hf response: %w", err)
	}

	meta := buildMetadata(ref, &info)
	return meta, nil
}

func buildMetadata(ref Ref, info *hubModelInfo) *Metadata {
	m := &Metadata{
		Model:       "hf:" + ref.RepoID(),
		Source:      "huggingface",
		Namespace:   ref.Namespace,
		Name:        ref.Name,
		Revision:    ref.Revision,
		PipelineTag: info.PipelineTag,
		LibraryName: info.LibraryName,
		ModelType:   info.Config.ModelType,
		Tags:        info.Tags,
		Private:     info.Private,
	}
	m.Gated = gatedTrue(info.Gated)
	m.License = licenseFrom(info)

	// Parameter estimate: prefer safetensors total, else infer from the name.
	if info.SafeTensors.Total > 0 {
		m.EstimatedParameters = info.SafeTensors.Total
	} else {
		m.EstimatedParameters = EstimateParamsFromName(ref.Name)
	}

	m.RecommendedRuntime, m.RequiresGPU = recommendRuntime(m)
	return m
}

func gatedTrue(v any) bool {
	switch t := v.(type) {
	case bool:
		return t
	case string:
		return t != "" && t != "false"
	default:
		return false
	}
}

func licenseFrom(info *hubModelInfo) string {
	switch l := info.CardData.License.(type) {
	case string:
		return l
	case []any:
		if len(l) > 0 {
			if s, ok := l[0].(string); ok {
				return s
			}
		}
	}
	for _, t := range info.Tags {
		if strings.HasPrefix(t, "license:") {
			return strings.TrimPrefix(t, "license:")
		}
	}
	return ""
}

var paramSizeRe = regexp.MustCompile(`(?i)(\d+(?:\.\d+)?)\s*([bm])\b`)

// EstimateParamsFromName infers a parameter count from a model name such as
// "Qwen2.5-7B-Instruct" (7B) or "phi-3-mini-128k" (no match -> 0).
func EstimateParamsFromName(name string) int64 {
	matches := paramSizeRe.FindAllStringSubmatch(name, -1)
	var best float64
	var unit string
	for _, mm := range matches {
		// Skip context-window-like tokens (e.g. 128k handled by 'k', not matched here).
		val, err := strconv.ParseFloat(mm[1], 64)
		if err != nil {
			continue
		}
		if val > best {
			best = val
			unit = strings.ToLower(mm[2])
		}
	}
	switch unit {
	case "b":
		return int64(best * 1e9)
	case "m":
		return int64(best * 1e6)
	}
	return 0
}

// recommendRuntime picks a serving runtime and GPU requirement based on the
// metadata. The heuristic is intentionally simple for the MVP.
func recommendRuntime(m *Metadata) (string, bool) {
	gguf := false
	for _, t := range m.Tags {
		if strings.Contains(strings.ToLower(t), "gguf") {
			gguf = true
		}
	}
	if gguf {
		return "ollama", false
	}
	if m.EstimatedParameters >= 7_000_000_000 {
		return "vllm", true
	}
	if m.EstimatedParameters > 0 && m.EstimatedParameters < 3_000_000_000 {
		return "ollama", false
	}
	return "vllm", m.EstimatedParameters >= 7_000_000_000
}
