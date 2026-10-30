# jirachecker

Finds feature branches in a GitHub repository whose Jira issues are closed, so they can be deleted.

A branch is a candidate when its name looks like `prefix/rest`, where `prefix` is one of
`feat, feature, bug, bugfix, fix, hotfix, chore, task` (case-insensitive, configurable with
`-prefixes`) and `rest` contains a Jira key such as `ABC-123` or `abc-123`. A branch is
**deletable** when every Jira key in its name is in the *Done* status category (Done, Closed,
Resolved, …). The default branch and protected branches are never listed.

## Build

Prerequisites:

- [Go](https://go.dev/dl/) 1.23 or newer (`go version` to check). On Windows: `winget install --id GoLang.Go -e`
- [GitHub CLI](https://cli.github.com/) (`gh`), needed at runtime for the GitHub token

The project uses only the Go standard library, so there is nothing to download besides Go itself.

Bash / zsh:

```sh
git clone <repo-url> jirachecker
cd jirachecker
go test ./...                 # run the unit tests
go build -o jirachecker .     # produces ./jirachecker
```

PowerShell:

```powershell
git clone <repo-url> jirachecker
cd jirachecker
go test ./...                    # run the unit tests
go build -o jirachecker.exe .    # produces .\jirachecker.exe
```

To install it onto your PATH instead (into `$(go env GOPATH)/bin`), run `go install .` from the
repository root.

### Building for another OS

Set `GOOS` and `GOARCH` to cross-compile, e.g. a Linux binary from any machine:

```sh
GOOS=linux GOARCH=amd64 go build -o jirachecker-linux-amd64 .
```

```powershell
$env:GOOS = "linux"; $env:GOARCH = "amd64"; go build -o jirachecker-linux-amd64 .
Remove-Item Env:GOOS, Env:GOARCH   # reset, otherwise later builds also target Linux
```

Common targets: `windows/amd64`, `linux/amd64`, `darwin/arm64` (Apple Silicon), `darwin/amd64`.

## Setup

Bash / zsh:

```sh
gh auth login                       # GitHub token is read via `gh auth token`
export JIRA_URL=https://yourcompany.atlassian.net
export JIRA_API_TOKEN=...           # Cloud: API token; Server/DC: personal access token
export JIRA_EMAIL=you@company.com   # Cloud only
```

PowerShell:

```powershell
gh auth login                                        # GitHub token is read via `gh auth token`
$env:JIRA_URL = "https://yourcompany.atlassian.net"
$env:JIRA_API_TOKEN = "..."                          # Cloud: API token; Server/DC: personal access token
$env:JIRA_EMAIL = "you@company.com"                  # Cloud only
```

`$env:` variables last for the current session only. To keep them for your user account, use
`[Environment]::SetEnvironmentVariable("JIRA_URL", "https://yourcompany.atlassian.net", "User")`
(and likewise for the others), then open a new terminal.

## Usage

Bash / zsh:

```sh
./jirachecker list owner/repo          # deletable branches
./jirachecker list owner/repo -all     # also show kept branches and why
```

PowerShell:

```powershell
.\jirachecker.exe list owner/repo      # deletable branches
.\jirachecker.exe list owner/repo -all # also show kept branches and why
```

Deleting branches is not implemented yet.
