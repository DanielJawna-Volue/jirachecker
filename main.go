// Command jirachecker finds GitHub feature branches whose Jira issues are closed.
package main

import (
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/volue/jirachecker/internal/branches"
	"github.com/volue/jirachecker/internal/github"
	"github.com/volue/jirachecker/internal/jira"
)

const usage = `jirachecker finds GitHub feature branches whose Jira issues are closed.

Usage:
  jirachecker list <owner/repo> [flags]

Environment:
  JIRA_URL        Jira base URL, e.g. https://yourcompany.atlassian.net
  JIRA_API_TOKEN  Jira Cloud API token, or Server/Data Center personal access token
  JIRA_EMAIL      Jira Cloud account email (leave unset for a Server/DC token)

The GitHub token is taken from the gh CLI (gh auth token).
Run 'jirachecker list -h' for the list flags.
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	switch os.Args[1] {
	case "list":
		os.Exit(runList(os.Args[2:]))
	case "-h", "--help", "help":
		fmt.Print(usage)
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n%s", os.Args[1], usage)
		os.Exit(2)
	}
}

type listOptions struct {
	repo       string
	prefixes   string
	showAll    bool
	jiraURL    string
	jiraEmail  string
	githubHost string
}

func runList(args []string) int {
	var o listOptions
	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	fs.StringVar(&o.prefixes, "prefixes", strings.Join(branches.DefaultPrefixes, ","), "comma separated branch prefixes to consider")
	fs.BoolVar(&o.showAll, "all", false, "also show feature branches that are not deletable, with the reason")
	fs.StringVar(&o.jiraURL, "jira-url", os.Getenv("JIRA_URL"), "Jira base URL (env JIRA_URL)")
	fs.StringVar(&o.jiraEmail, "jira-email", os.Getenv("JIRA_EMAIL"), "Jira Cloud account email (env JIRA_EMAIL)")
	fs.StringVar(&o.githubHost, "github-host", "github.com", "gh host to take the token from")
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "Usage: jirachecker list <owner/repo> [flags]")
		fs.PrintDefaults()
	}

	positional, err := parseInterspersed(fs, args)
	if err != nil {
		return 2
	}
	if len(positional) != 1 {
		fs.Usage()
		return 2
	}
	o.repo = positional[0]

	if err := list(o); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	return 0
}

// parseInterspersed lets flags appear before or after positional arguments,
// which the standard flag package does not allow on its own.
func parseInterspersed(fs *flag.FlagSet, args []string) ([]string, error) {
	var positional []string
	for {
		if err := fs.Parse(args); err != nil {
			return nil, err
		}
		if fs.NArg() == 0 {
			return positional, nil
		}
		positional = append(positional, fs.Arg(0))
		args = fs.Args()[1:]
	}
}

func list(o listOptions) error {
	jiraToken := os.Getenv("JIRA_API_TOKEN")
	if o.jiraURL == "" || jiraToken == "" {
		return fmt.Errorf("Jira is not configured: set JIRA_URL (or -jira-url) and JIRA_API_TOKEN, plus JIRA_EMAIL for Jira Cloud")
	}

	repoName, err := github.ParseRepo(o.repo)
	if err != nil {
		return err
	}
	token, err := github.Token(o.githubHost)
	if err != nil {
		return err
	}
	gh := github.NewClient(token)
	repo, err := gh.GetRepo(repoName)
	if err != nil {
		return err
	}

	jc := jira.NewClient(o.jiraURL, jiraToken, o.jiraEmail)
	jiraUser, err := jc.Verify()
	if err != nil {
		return err
	}
	fmt.Printf("Repository: %s (default branch: %s)\n", repo.FullName, repo.DefaultBranch)
	fmt.Printf("Jira:       %s as %s\n\n", o.jiraURL, jiraUser)

	var prefixes []string
	for _, p := range strings.Split(o.prefixes, ",") {
		if p = strings.TrimSpace(p); p != "" {
			prefixes = append(prefixes, p)
		}
	}

	all, err := gh.ListBranches(repoName)
	if err != nil {
		return err
	}
	var features []*branches.FeatureBranch
	keySet := map[string]bool{}
	for _, b := range all {
		if b.Name == repo.DefaultBranch || b.Protected {
			continue
		}
		if fb := branches.Parse(b.Name, prefixes); fb != nil {
			features = append(features, fb)
			for _, k := range fb.IssueKeys {
				keySet[k] = true
			}
		}
	}
	sort.Slice(features, func(i, j int) bool {
		return strings.ToLower(features[i].Name) < strings.ToLower(features[j].Name)
	})

	keys := make([]string, 0, len(keySet))
	for k := range keySet {
		keys = append(keys, k)
	}
	statuses := jc.Statuses(keys, 8)

	var deletable, kept [][2]string
	for _, fb := range features {
		ok, reason := verdict(fb, statuses)
		if ok {
			deletable = append(deletable, [2]string{fb.Name, reason})
		} else {
			kept = append(kept, [2]string{fb.Name, reason})
		}
	}

	if len(deletable) > 0 {
		fmt.Printf("Deletable branches (%d):\n", len(deletable))
		printTable(deletable, "BRANCH", "JIRA STATUS")
	} else {
		fmt.Println("No deletable branches found.")
	}
	if o.showAll && len(kept) > 0 {
		fmt.Printf("\nBranches to keep (%d):\n", len(kept))
		printTable(kept, "BRANCH", "REASON")
	}
	fmt.Printf("\nScanned %d branches, %d feature branches, %d Jira issues; %d deletable.\n",
		len(all), len(features), len(keys), len(deletable))
	return nil
}

// verdict reports whether a branch is deletable: it must reference at least one issue,
// and every referenced issue must be closed.
func verdict(fb *branches.FeatureBranch, statuses map[string]jira.IssueStatus) (bool, string) {
	if len(fb.IssueKeys) == 0 {
		return false, "no Jira issue key in name"
	}
	ok := true
	parts := make([]string, 0, len(fb.IssueKeys))
	for _, k := range fb.IssueKeys {
		s := statuses[k]
		if s.Found {
			parts = append(parts, k+": "+s.Status)
		} else {
			parts = append(parts, k+": "+s.Err)
		}
		ok = ok && s.Closed()
	}
	return ok, strings.Join(parts, ", ")
}

func printTable(rows [][2]string, h1, h2 string) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "%s\t%s\n%s\t%s\n", h1, h2, strings.Repeat("-", len(h1)), strings.Repeat("-", len(h2)))
	for _, r := range rows {
		fmt.Fprintf(w, "%s\t%s\n", r[0], r[1])
	}
	w.Flush()
}
