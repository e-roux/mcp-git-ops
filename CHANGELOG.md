# Changelog

## 0.2.1

- Patch release to trigger Go module proxy cache refresh

## 0.2.0

- Refactored: moved main.go from cmd/mcp-git-ops/ to root for simpler go install
- Bumped minor version for breaking change in project structure

## 0.1.2

- Fixed module path to match repository URL (github.com/e-roux/mcp-git-ops)

## 0.1.1

- Refactored project to follow standard Go layout: main package under `cmd/mcp-git-ops` and internal packages under `internal/`
- Standardized package naming and visibility
- Updated build and install targets to build the command module
- Omitted comment lines in Go source files to ensure clean and self-documenting code
- Updated installation documentation for alternate GOBIN usage

## 0.1.0

- MCP server with push, create_pr, merge_pr, pr_status tools
- Platform adapters: GitHub (gh), GitLab (glab), Azure DevOps (az repos)
- Auto-detect platform from git remote URL
- Branch protection enforcement in push and create_pr
- Environment variable overrides for platform and protected branches
