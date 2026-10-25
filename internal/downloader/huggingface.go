package downloader

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
)

// HFAPIBase is the Hugging Face API root. Overridable in tests.
var HFAPIBase = "https://huggingface.co"

// HFModel is a search result from the Hugging Face models API.
type HFModel struct {
	ID        string `json:"id"`
	Downloads int    `json:"downloads"`
	Likes     int    `json:"likes"`
}

// HFFile is one file in a Hugging Face model repo.
type HFFile struct {
	Path string `json:"rfilename"`
}

// SearchGGUF searches Hugging Face for GGUF model repos matching query,
// sorted by downloads descending.
func SearchGGUF(ctx context.Context, client *http.Client, query string, limit int) ([]HFModel, error) {
	if client == nil {
		client = http.DefaultClient
	}
	url := fmt.Sprintf("%s/api/models?search=%s&filter=gguf&limit=%d&sort=downloads&direction=-1",
		HFAPIBase, urlEscape(query), limit)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("huggingface search: status %d", resp.StatusCode)
	}
	var models []HFModel
	if err := json.NewDecoder(resp.Body).Decode(&models); err != nil {
		return nil, fmt.Errorf("decode search results: %w", err)
	}
	sort.Slice(models, func(i, j int) bool { return models[i].Downloads > models[j].Downloads })
	return models, nil
}

// ListGGUFFiles lists the .gguf files available in a Hugging Face repo.
func ListGGUFFiles(ctx context.Context, client *http.Client, repo string) ([]string, error) {
	if client == nil {
		client = http.DefaultClient
	}
	url := fmt.Sprintf("%s/api/models/%s", HFAPIBase, repo)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("huggingface repo info: status %d", resp.StatusCode)
	}
	var info struct {
		Siblings []HFFile `json:"siblings"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, fmt.Errorf("decode repo info: %w", err)
	}
	var out []string
	for _, f := range info.Siblings {
		if strings.HasSuffix(f.Path, ".gguf") {
			out = append(out, f.Path)
		}
	}
	return out, nil
}

// FileURL builds the direct download URL for a file within a repo.
func FileURL(repo, file string) string {
	return fmt.Sprintf("%s/%s/resolve/main/%s", HFAPIBase, repo, file)
}

func urlEscape(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, " ", "+"), "#", "%23")
}
