package github

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client is a GitHub API client for fetching repository contents.
type Client struct {
	httpClient *http.Client
	baseURL    string
}

// NewClient creates a new GitHub API client.
func NewClient() *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		baseURL: "https://api.github.com",
	}
}

// TreeEntry represents a file or directory in a GitHub repository tree.
type TreeEntry struct {
	Path string `json:"path"`
	Mode string `json:"mode"`
	Type string `json:"type"` // "blob" or "tree"
	SHA  string `json:"sha"`
	Size int64  `json:"size,omitempty"`
	URL  string `json:"url,omitempty"`
}

// Tree represents the response from the GitHub Trees API.
type Tree struct {
	SHA       string      `json:"sha"`
	URL       string      `json:"url"`
	Tree      []TreeEntry `json:"tree"`
	Truncated bool        `json:"truncated"`
}

// FileContent represents the response from the GitHub Contents API.
type FileContent struct {
	Type        string `json:"type"`
	Encoding    string `json:"encoding"`
	Size        int64  `json:"size"`
	Name        string `json:"name"`
	Path        string `json:"path"`
	Content     string `json:"content"`
	SHA         string `json:"sha"`
	URL         string `json:"url"`
	DownloadURL string `json:"download_url"`
}

// Branch represents a GitHub branch.
type Branch struct {
	Name   string `json:"name"`
	Commit struct {
		SHA string `json:"sha"`
		URL string `json:"url"`
	} `json:"commit"`
}

// GetTree fetches the repository tree (recursive) for a given branch.
func (c *Client) GetTree(ctx context.Context, owner, repo, branch, token string) (*Tree, error) {
	// First, get the branch to find the commit SHA
	branchInfo, err := c.GetBranch(ctx, owner, repo, branch, token)
	if err != nil {
		return nil, fmt.Errorf("failed to get branch: %w", err)
	}

	// Then get the tree recursively
	url := fmt.Sprintf("%s/repos/%s/%s/git/trees/%s?recursive=1", c.baseURL, owner, repo, branchInfo.Commit.SHA)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch tree: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GitHub API error (status %d): %s", resp.StatusCode, string(body))
	}

	var tree Tree
	if err := json.NewDecoder(resp.Body).Decode(&tree); err != nil {
		return nil, fmt.Errorf("failed to decode tree response: %w", err)
	}

	return &tree, nil
}

// GetBranch fetches information about a branch.
func (c *Client) GetBranch(ctx context.Context, owner, repo, branch, token string) (*Branch, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/branches/%s", c.baseURL, owner, repo, branch)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch branch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GitHub API error (status %d): %s", resp.StatusCode, string(body))
	}

	var branchInfo Branch
	if err := json.NewDecoder(resp.Body).Decode(&branchInfo); err != nil {
		return nil, fmt.Errorf("failed to decode branch response: %w", err)
	}

	return &branchInfo, nil
}

// GetFileContent fetches the content of a file from the repository.
func (c *Client) GetFileContent(ctx context.Context, owner, repo, path, ref, token string) ([]byte, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/contents/%s?ref=%s", c.baseURL, owner, repo, path, ref)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch file: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrFileNotFound
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GitHub API error (status %d): %s", resp.StatusCode, string(body))
	}

	var fileContent FileContent
	if err := json.NewDecoder(resp.Body).Decode(&fileContent); err != nil {
		return nil, fmt.Errorf("failed to decode file response: %w", err)
	}

	// Decode base64 content
	if fileContent.Encoding != "base64" {
		return nil, fmt.Errorf("unexpected encoding: %s", fileContent.Encoding)
	}

	content, err := base64.StdEncoding.DecodeString(fileContent.Content)
	if err != nil {
		return nil, fmt.Errorf("failed to decode base64 content: %w", err)
	}

	return content, nil
}

// GetRawFileContent fetches raw file content using the download URL.
// This is better for larger files or binary content.
func (c *Client) GetRawFileContent(ctx context.Context, owner, repo, path, ref, token string) ([]byte, error) {
	url := fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s/%s", owner, repo, ref, path)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch raw file: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrFileNotFound
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub raw content error (status %d)", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}

// TestConnection tests if the token can access the repository.
func (c *Client) TestConnection(ctx context.Context, owner, repo, branch, token string) error {
	_, err := c.GetBranch(ctx, owner, repo, branch, token)
	return err
}

// Error types
var (
	ErrFileNotFound = fmt.Errorf("file not found")
)
