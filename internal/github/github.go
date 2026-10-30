// Package github is a minimal GitHub REST client authenticated with the gh CLI token.
package github

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

const apiURL = "https://api.github.com"

var (
	repoRe       = regexp.MustCompile(`^[A-Za-z0-9-]+/[A-Za-z0-9._-]+$`)
	repoPrefixRe = regexp.MustCompile(`^(https?://github\.com/|git@github\.com:)`)
	nextLinkRe   = regexp.MustCompile(`<([^>]+)>;\s*rel="next"`)

	errNotFound = errors.New("not found")
)

// Token returns the current user's token as reported by `gh auth token`.
func Token(hostname string) (string, error) {
	if _, err := exec.LookPath("gh"); err != nil {
		return "", errors.New("the GitHub CLI (gh) is not installed or not on PATH")
	}
	cmd := exec.Command("gh", "auth", "token", "--hostname", hostname)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	token := strings.TrimSpace(string(out))
	if err != nil || token == "" {
		return "", fmt.Errorf("could not obtain a token from gh; run `gh auth login` first: %s",
			strings.TrimSpace(stderr.String()))
	}
	return token, nil
}

// ParseRepo normalises owner/repo, https://github.com/owner/repo(.git) or git@github.com:owner/repo.git.
func ParseRepo(value string) (string, error) {
	repo := repoPrefixRe.ReplaceAllString(strings.TrimSpace(value), "")
	repo = strings.TrimSuffix(strings.TrimSuffix(repo, "/"), ".git")
	if !repoRe.MatchString(repo) {
		return "", fmt.Errorf("%q is not a valid repository; expected the form owner/repo", value)
	}
	return repo, nil
}

// Repo holds the repository fields we use.
type Repo struct {
	FullName      string `json:"full_name"`
	DefaultBranch string `json:"default_branch"`
}

// Branch holds the branch fields we use.
type Branch struct {
	Name      string `json:"name"`
	Protected bool   `json:"protected"`
}

// Client talks to the GitHub REST API.
type Client struct {
	token string
	http  *http.Client
}

func NewClient(token string) *Client {
	return &Client{token: token, http: &http.Client{Timeout: 30 * time.Second}}
}

// get fetches url into v and returns the Link header.
func (c *Client) get(url string, v any) (string, error) {
	if !strings.HasPrefix(url, "http") {
		url = apiURL + url
	}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("could not reach GitHub: %w", err)
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == http.StatusNotFound:
		return "", errNotFound
	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("GitHub denied access (%d) to %s: %s", resp.StatusCode, url, body)
	case resp.StatusCode >= 300:
		return "", fmt.Errorf("GitHub request failed (%d) for %s", resp.StatusCode, url)
	}
	if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
		return "", fmt.Errorf("decoding GitHub response from %s: %w", url, err)
	}
	return resp.Header.Get("Link"), nil
}

// GetRepo returns the repository, or a readable error if it does not exist or is not accessible.
func (c *Client) GetRepo(repo string) (*Repo, error) {
	var r Repo
	if _, err := c.get("/repos/"+repo, &r); err != nil {
		if errors.Is(err, errNotFound) {
			return nil, fmt.Errorf("repository %q does not exist or you do not have access to it", repo)
		}
		return nil, err
	}
	return &r, nil
}

// ListBranches returns all branches of repo, following pagination.
func (c *Client) ListBranches(repo string) ([]Branch, error) {
	var all []Branch
	url := "/repos/" + repo + "/branches?per_page=100"
	for url != "" {
		var page []Branch
		link, err := c.get(url, &page)
		if err != nil {
			return nil, err
		}
		all = append(all, page...)
		url = ""
		if m := nextLinkRe.FindStringSubmatch(link); m != nil {
			url = m[1]
		}
	}
	return all, nil
}
