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
  jirachecker list  <owner/repo> [flags]                  report on one repository
  jirachecker multi <owner/repo> <owner/repo>... [flags]  report on several repositories

Examples:
  jirachecker list  volue/api
  jirachecker multi volue/api volue/web volue/worker
  jirachecker multi volue/api,volue/web -all

Environment:
  JIRA_URL        Jira base URL, e.g. https://yourcompany.atlassian.net
  JIRA_API_TOKEN  Jira Cloud API token, or Server/Data Center personal access token
  JIRA_EMAIL      Jira Cloud account email (leave unset for a Server/DC token)

The GitHub token is taken from the gh CLI (gh auth token).
Run 'jirachecker list -h' or 'jirachecker multi -h' for the flags.
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	switch os.Args[1] {
	case "list":
		os.Exit(runList(os.Args[2:]))
	case "multi":
		os.Exit(runMulti(os.Args[2:]))
	case "-h", "--help", "help":
		fmt.Print(usage)
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n%s", os.Args[1], usage)
		os.Exit(2)
	}
}

type listOptions struct {
	prefixes   string
	showAll    bool
	jiraURL    string
	jiraEmail  string
	githubHost string
}

// newFlagSet returns the flags shared by the list and multi commands.
func newFlagSet(name, usageLine, examples string, o *listOptions) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.StringVar(&o.prefixes, "prefixes", strings.Join(branches.DefaultPrefixes, ","), "comma separated branch prefixes to consider")
	fs.BoolVar(&o.showAll, "all", false, "also show feature branches that are not deletable, with the reason")
	fs.StringVar(&o.jiraURL, "jira-url", os.Getenv("JIRA_URL"), "Jira base URL (env JIRA_URL)")
	fs.StringVar(&o.jiraEmail, "jira-email", os.Getenv("JIRA_EMAIL"), "Jira Cloud account email (env JIRA_EMAIL)")
	fs.StringVar(&o.githubHost, "github-host", "github.com", "gh host to take the token from")
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "Usage: "+usageLine)
		fmt.Fprintln(fs.Output(), "\nExamples:\n"+examples+"\nFlags:")
		fs.PrintDefaults()
	}
	return fs
}

func runList(args []string) int {
	var o listOptions
	fs := newFlagSet("list", "jirachecker list <owner/repo> [flags]",
		"  jirachecker list volue/api\n  jirachecker list volue/api -all\n", &o)
	positional, err := parseInterspersed(fs, args)
	if err != nil {
		return 2
	}
	if len(positional) != 1 {
		fs.Usage()
		return 2
	}

	gh, jc, err := connect(o)
	if err == nil {
		_, err = scanRepo(gh, jc, positional[0], o)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	return 0
}

func runMulti(args []string) int {
	var o listOptions
	fs := newFlagSet("multi", "jirachecker multi <owner/repo> <owner/repo>... [flags]\n\n"+
		"Repositories are separated by spaces and/or commas. A repository that fails\n"+
		"(not found, no access) is reported and the others are still scanned.",
		"  jirachecker multi volue/api volue/web volue/worker\n"+
			"  jirachecker multi volue/api,volue/web,volue/worker\n"+
			"  jirachecker multi volue/api volue/web -all -prefixes feat,fix\n", &o)
	positional, err := parseInterspersed(fs, args)
	if err != nil {
		return 2
	}
	var repos []string
	seen := map[string]bool{}
	for _, arg := range positional {
		for _, r := range strings.Split(arg, ",") {
			if r = strings.TrimSpace(r); r != "" && !seen[strings.ToLower(r)] {
				seen[strings.ToLower(r)] = true
				repos = append(repos, r)
			}
		}
	}
	if len(repos) == 0 {
		fs.Usage()
		return 2
	}

	gh, jc, err := connect(o)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}

	var summary [][2]string
	failed := 0
	for i, r := range repos {
		if i > 0 {
			fmt.Println()
		}
		fmt.Printf("=== %s ===\n", r)
		n, err := scanRepo(gh, jc, r, o)
		if err != nil {
			failed++
			fmt.Fprintln(os.Stderr, "error:", err)
			summary = append(summary, [2]string{r, "error: " + err.Error()})
			continue
		}
		summary = append(summary, [2]string{r, fmt.Sprint(n)})
	}

	fmt.Printf("\nSummary (%d repositories):\n", len(repos))
	printTable(summary, "REPOSITORY", "DELETABLE")
	if failed > 0 {
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

// connect creates the GitHub and Jira clients and checks the Jira credentials.
func connect(o listOptions) (*github.Client, *jira.Client, error) {
	jiraToken := os.Getenv("JIRA_API_TOKEN")
	if o.jiraURL == "" || jiraToken == "" {
		return nil, nil, fmt.Errorf("Jira is not configured: set JIRA_URL (or -jira-url) and JIRA_API_TOKEN, plus JIRA_EMAIL for Jira Cloud")
	}
	token, err := github.Token(o.githubHost)
	if err != nil {
		return nil, nil, err
	}
	jc := jira.NewClient(o.jiraURL, jiraToken, o.jiraEmail)
	jiraUser, err := jc.Verify()
	if err != nil {
		return nil, nil, err
	}
	fmt.Printf("Jira:       %s as %s\n", o.jiraURL, jiraUser)
	return github.NewClient(token), jc, nil
}

// scanRepo prints the report for one repository and returns the number of deletable branches.
func scanRepo(gh *github.Client, jc *jira.Client, repoArg string, o listOptions) (int, error) {
	repoName, err := github.ParseRepo(repoArg)
	if err != nil {
		return 0, err
	}
	repo, err := gh.GetRepo(repoName)
	if err != nil {
		return 0, err
	}
	fmt.Printf("Repository: %s (default branch: %s)\n\n", repo.FullName, repo.DefaultBranch)

	var prefixes []string
	for _, p := range strings.Split(o.prefixes, ",") {
		if p = strings.TrimSpace(p); p != "" {
			prefixes = append(prefixes, p)
		}
	}

	all, err := gh.ListBranches(repoName)
	if err != nil {
		return 0, err
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
	return len(deletable), nil
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
