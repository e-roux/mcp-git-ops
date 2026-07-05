# mcp-git-ops

A lightweight MCP (Model Context Protocol) server providing platform-agnostic
git operations: push, pull/merge request creation, merge, and status.

Supports GitHub (`gh`), GitLab (`glab`), and Azure DevOps (`az repos`) through
a unified tool interface. Auto-detects the platform from the git remote URL.

## Installation

To install the latest version to an alternate `GOBIN` (such as `XDG_BIN_HOME` or `$HOME/.local/bin`):

```bash
GOBIN=${XDG_BIN_HOME:-$HOME/.local/bin} go install github.com/e-roux/mcp-git-ops/cmd/mcp-git-ops@latest
```

Or build from source:

```bash
make build
```

## Configuration

Add to `.mcp.json` at the project root:

```json
{
  "mcpServers": {
    "git-ops": {
      "command": "mcp-git-ops"
    }
  }
}
```

Works with both GitHub Copilot CLI and Claude Code.

## Tools

| Tool | Description |
|------|-------------|
| `push` | Push commits to remote with branch protection |
| `create_pr` | Create a pull/merge request |
| `merge_pr` | Merge a pull/merge request |
| `pr_status` | Get PR/MR status |

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `PROTECTED_BRANCHES` | `main,master` | Comma-separated protected branch names |
| `GIT_OPS_PLATFORM` | auto-detect | Force platform: `github`, `gitlab`, `azuredevops` |

## Prerequisites

At least one platform CLI must be installed and authenticated:

- `gh` CLI (GitHub)
- `glab` CLI (GitLab)
- `az` CLI with repos extension (Azure DevOps)

## Development

```bash
make qa       # run tests + vet
make build    # build binary
make install  # go install
```
