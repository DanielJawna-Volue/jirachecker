// Package jira is a minimal Jira REST client for Jira Cloud and Jira Server/Data Center.
package jira

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"
)

// IssueStatus is the outcome of looking up one issue.
type IssueStatus struct {
	Key      string
	Found    bool
	Status   string // e.g. "Done", "Closed", "In Progress"
	Category string // Jira status category key: "new", "indeterminate" or "done"
	Err      string // why the lookup failed, when !Found
}

// Closed reports whether the issue is in the "done" status category, which covers
// Done, Closed, Resolved and any custom final status.
func (s IssueStatus) Closed() bool { return s.Found && s.Category == "done" }

// Client talks to the Jira REST API.
type Client struct {
	baseURL string
	auth    string
	http    *http.Client
}

// NewClient creates a client. With an email the token is a Jira Cloud API token (basic auth);
// without one it is a Server/Data Center personal access token (bearer auth).
func NewClient(baseURL, token, email string) *Client {
	auth := "Bearer " + token
	if email != "" {
		auth = "Basic " + base64.StdEncoding.EncodeToString([]byte(email+":"+token))
	}
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		auth:    auth,
		http:    &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *Client) get(path string, v any) (int, error) {
	req, err := http.NewRequest(http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Authorization", c.auth)
	req.Header.Set("Accept", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return resp.StatusCode, nil
	}
	return resp.StatusCode, json.NewDecoder(resp.Body).Decode(v)
}

// Verify checks the credentials and returns the authenticated user's display name.
func (c *Client) Verify() (string, error) {
	var me struct {
		DisplayName string `json:"displayName"`
		Name        string `json:"name"`
	}
	code, err := c.get("/rest/api/2/myself", &me)
	switch {
	case err != nil:
		return "", fmt.Errorf("could not reach Jira at %s: %w", c.baseURL, err)
	case code == http.StatusUnauthorized || code == http.StatusForbidden:
		return "", fmt.Errorf("Jira rejected the credentials (%d)", code)
	case code >= 300:
		return "", fmt.Errorf("Jira request failed (%d) for %s", code, c.baseURL)
	}
	if me.DisplayName != "" {
		return me.DisplayName, nil
	}
	return me.Name, nil
}

// Status looks up a single issue's status. Failures are reported in the result, not as an error,
// so one bad key does not abort the whole run.
func (c *Client) Status(key string) IssueStatus {
	var issue struct {
		Fields struct {
			Status struct {
				Name           string `json:"name"`
				StatusCategory struct {
					Key string `json:"key"`
				} `json:"statusCategory"`
			} `json:"status"`
		} `json:"fields"`
	}
	code, err := c.get("/rest/api/2/issue/"+url.PathEscape(key)+"?fields=status", &issue)
	switch {
	case err != nil:
		return IssueStatus{Key: key, Err: err.Error()}
	case code == http.StatusNotFound:
		return IssueStatus{Key: key, Err: "issue not found"}
	case code >= 300:
		return IssueStatus{Key: key, Err: fmt.Sprintf("HTTP %d", code)}
	}
	s := issue.Fields.Status
	return IssueStatus{Key: key, Found: true, Status: s.Name, Category: s.StatusCategory.Key}
}

// Statuses looks up many issues concurrently.
func (c *Client) Statuses(keys []string, workers int) map[string]IssueStatus {
	sorted := append([]string(nil), keys...)
	sort.Strings(sorted)

	results := make(map[string]IssueStatus, len(sorted))
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, workers)
	for _, k := range sorted {
		wg.Add(1)
		sem <- struct{}{}
		go func(k string) {
			defer func() { <-sem; wg.Done() }()
			s := c.Status(k)
			mu.Lock()
			results[k] = s
			mu.Unlock()
		}(k)
	}
	wg.Wait()
	return results
}
