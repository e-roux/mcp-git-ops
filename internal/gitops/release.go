package gitops

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/e-roux/agent-plugins/plugins/dev/mcp-git-ops/internal/platform"
	"github.com/mark3labs/mcp-go/mcp"
)

func releaseStatusTool() mcp.Tool {
	return mcp.NewTool("release_status",
		mcp.WithDescription("Read-only release readiness report: clean tree, latest tag, changelog validity, inferred next version, CI detection."),
		mcp.WithString("cwd", mcp.Description("Working directory of the git repository.")),
	)
}

func createReleaseTool() mcp.Tool {
	return mcp.NewTool("create_release",
		mcp.WithDescription("Create a platform release for an already-pushed tag. Auto-detects GitHub/GitLab. Azure DevOps is not supported."),
		mcp.WithString("cwd", mcp.Description("Working directory of the git repository.")),
		mcp.WithString("tag", mcp.Required(), mcp.Description("Tag that has already been pushed to origin.")),
		mcp.WithString("title", mcp.Description("Release title. Defaults to the tag name.")),
		mcp.WithString("notes", mcp.Required(), mcp.Description("Release notes (markdown). Extract from CHANGELOG.md for this version.")),
		mcp.WithBoolean("draft", mcp.Description("Create as a draft release.")),
		mcp.WithBoolean("prerelease", mcp.Description("Mark as a pre-release.")),
		mcp.WithString("target", mcp.Description("Target commitish. Defaults to the default branch.")),
	)
}

func handleReleaseStatus(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	cwd := stringArg(request, "cwd", "")

	var lines []string

	branch, err := platform.CurrentBranch(cwd)
	if err != nil {
		lines = append(lines, fmt.Sprintf("branch: unknown (%s)", err))
	} else {
		lines = append(lines, fmt.Sprintf("branch: %s", branch))
	}

	dirty, err := platform.RunGitCommand(cwd, "status", "--porcelain")
	if err != nil || dirty != "" {
		lines = append(lines, "tree: dirty — commit or stash changes before releasing")
	} else {
		lines = append(lines, "tree: clean")
	}

	latestTag, err := platform.RunGitCommand(cwd, "describe", "--tags", "--abbrev=0")
	if err != nil || latestTag == "" {
		lines = append(lines, "latest-tag: none")
		latestTag = ""
	} else {
		lines = append(lines, fmt.Sprintf("latest-tag: %s", latestTag))
	}

	dir := cwd
	if dir == "" {
		dir, _ = os.Getwd()
	}

	changelogPath := dir + "/CHANGELOG.md"
	clStatus, unreleased, inferred := InspectChangelog(changelogPath, latestTag)
	lines = append(lines, fmt.Sprintf("changelog: %s", clStatus))
	if unreleased != "" {
		lines = append(lines, fmt.Sprintf("unreleased-categories: %s", unreleased))
	}
	if inferred != "" {
		lines = append(lines, fmt.Sprintf("inferred-next-version: %s (confirm with user before tagging)", inferred))
	}

	ciStatus := detectCI(dir)
	lines = append(lines, fmt.Sprintf("ci: %s", ciStatus))

	lines = append(lines, "")
	lines = append(lines, "Next step: follow the two-phase release workflow in the git-release skill resource.")

	return mcp.NewToolResultText(strings.Join(lines, "\n")), nil
}

func handleCreateRelease(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	cwd := stringArg(request, "cwd", "")
	tag := stringArg(request, "tag", "")
	title := stringArg(request, "title", tag)
	notes := stringArg(request, "notes", "")
	draft := boolArg(request, "draft")
	prerelease := boolArg(request, "prerelease")
	target := stringArg(request, "target", "")

	if tag == "" {
		return errorResult("tag is required"), nil
	}
	if notes == "" {
		return errorResult("notes is required — extract the relevant section from CHANGELOG.md"), nil
	}
	if title == "" {
		title = tag
	}

	localTags, err := platform.RunGitCommand(cwd, "tag", "-l", tag)
	if err != nil || strings.TrimSpace(localTags) == "" {
		return errorResult("tag %s does not exist locally — create it with: git tag -a %s -m 'Release %s'", tag, tag, tag), nil
	}

	remoteRefs, err := platform.RunGitCommand(cwd, "ls-remote", "--tags", "origin", tag)
	if err != nil || strings.TrimSpace(remoteRefs) == "" {
		return errorResult("tag %s not found on origin — push it first: git push origin %s", tag, tag), nil
	}

	plat, err := platform.DetectPlatform(cwd)
	if err != nil {
		return errorResult("platform detection failed: %s", err), nil
	}

	info, err := plat.CreateRelease(ctx, platform.ReleaseCreateOptions{
		Tag:        tag,
		Title:      title,
		Notes:      notes,
		Draft:      draft,
		Prerelease: prerelease,
		Target:     target,
	})
	if err != nil {
		return errorResult("create release failed (%s): %s", plat.PlatformName(), err), nil
	}

	output := fmt.Sprintf("Release %s created on %s", tag, plat.PlatformName())
	if info != nil && info.URL != "" {
		output = fmt.Sprintf("Release %s created on %s: %s", tag, plat.PlatformName(), info.URL)
	}
	return mcp.NewToolResultText(output), nil
}

func InspectChangelog(path string, latestTag string) (status string, categories string, inferredVersion string) {
	f, err := os.Open(path)
	if err != nil {
		return "CHANGELOG.md not found — create one with an [Unreleased] section", "", ""
	}
	defer f.Close()

	var hasUnreleased bool
	var unreleasedSections []string
	var inUnreleased bool

	sectionRe := regexp.MustCompile(`^### (Added|Changed|Deprecated|Removed|Fixed|Security)`)
	unreleasedHeadingRe := regexp.MustCompile(`^## \[Unreleased\]`)
	versionHeadingRe := regexp.MustCompile(`^## \[`)

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()

		if unreleasedHeadingRe.MatchString(line) {
			hasUnreleased = true
			inUnreleased = true
			continue
		}
		if inUnreleased {
			if versionHeadingRe.MatchString(line) {
				inUnreleased = false
				continue
			}
			if m := sectionRe.FindString(line); m != "" {
				sec := strings.TrimPrefix(m, "### ")
				unreleasedSections = append(unreleasedSections, sec)
			}
		}
	}

	if !hasUnreleased {
		return "missing [Unreleased] section — add one before writing changelog entries", "", ""
	}

	if len(unreleasedSections) == 0 {
		return "ok — [Unreleased] is empty (no entries to release)", "", ""
	}

	categories = strings.Join(dedup(unreleasedSections), ", ")
	status = fmt.Sprintf("ok — [Unreleased] has categories: %s", categories)
	inferredVersion = InferNextVersion(latestTag, unreleasedSections)
	return
}

func InferNextVersion(latestTag string, sections []string) string {
	if latestTag == "" {
		return "v1.0.0 (first release)"
	}

	sectionSet := make(map[string]bool)
	for _, s := range sections {
		sectionSet[s] = true
	}

	major, minor, patch := 0, 0, 0
	tag := strings.TrimPrefix(latestTag, "v")
	fmt.Sscanf(tag, "%d.%d.%d", &major, &minor, &patch)

	if sectionSet["Removed"] {
		return fmt.Sprintf("v%d.0.0", major+1)
	}
	if sectionSet["Added"] || sectionSet["Changed"] {
		return fmt.Sprintf("v%d.%d.0", major, minor+1)
	}
	return fmt.Sprintf("v%d.%d.%d", major, minor, patch+1)
}

func detectCI(dir string) string {
	if info, err := os.Stat(dir + "/.github/workflows"); err == nil && info.IsDir() {
		return "GitHub Actions workflows detected (verify runners are available)"
	}
	if _, err := os.Stat(dir + "/.gitlab-ci.yml"); err == nil {
		return "GitLab CI detected"
	}
	if _, err := os.Stat(dir + "/.gitlab/ci"); err == nil {
		return "GitLab CI detected (split config)"
	}
	return "no CI configuration detected — create the platform release locally via CLI or mcp__git-ops__create_release"
}

func dedup(in []string) []string {
	seen := make(map[string]bool)
	var out []string
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}
