// Package branches parses feature branch names into Jira issue keys.
package branches

import (
	"regexp"
	"strings"
)

// DefaultPrefixes are the branch prefixes considered feature branches by default.
var DefaultPrefixes = []string{"feat", "feature", "bug", "bugfix", "fix", "hotfix", "chore", "task"}

// Jira keys look like ABC-123 or abc-123; the project part starts with a letter.
// Go's regexp has no lookarounds, so boundaries are checked by hand in ExtractIssueKeys.
var issueKeyRe = regexp.MustCompile(`[A-Za-z][A-Za-z0-9_]*-[0-9]+`)

// FeatureBranch is a branch named prefix/rest.
type FeatureBranch struct {
	Name      string
	Prefix    string
	IssueKeys []string
}

// ExtractIssueKeys returns the upper-cased, de-duplicated Jira keys in text, in order of appearance.
func ExtractIssueKeys(text string) []string {
	var keys []string
	seen := map[string]bool{}
	for _, loc := range issueKeyRe.FindAllStringIndex(text, -1) {
		start, end := loc[0], loc[1]
		// Reject matches glued to a preceding alphanumeric, e.g. "ABC-2" inside "1ABC-2".
		if start > 0 && isAlnum(text[start-1]) {
			continue
		}
		key := strings.ToUpper(text[start:end])
		if !seen[key] {
			seen[key] = true
			keys = append(keys, key)
		}
	}
	return keys
}

// Parse returns the FeatureBranch for name if it looks like prefix/rest with a known prefix
// (case-insensitive), or nil otherwise. The returned branch may contain no issue keys.
func Parse(name string, prefixes []string) *FeatureBranch {
	prefix, rest, ok := strings.Cut(name, "/")
	if !ok || rest == "" {
		return nil
	}
	for _, p := range prefixes {
		if strings.EqualFold(p, prefix) {
			return &FeatureBranch{Name: name, Prefix: prefix, IssueKeys: ExtractIssueKeys(rest)}
		}
	}
	return nil
}

func isAlnum(c byte) bool {
	return c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}
