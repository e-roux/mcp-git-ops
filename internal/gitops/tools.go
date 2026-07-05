package gitops

import (
	"context"
	"fmt"

	"github.com/e-roux/mcp-git-ops/internal/platform"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func RegisterAllTools(s *server.MCPServer) {
	s.AddTool(pushTool(), handlePush)
	s.AddTool(createPRTool(), handleCreatePR)
	s.AddTool(mergePRTool(), handleMergePR)
	s.AddTool(prStatusTool(), handlePRStatus)
	s.AddTool(releaseStatusTool(), handleReleaseStatus)
	s.AddTool(createReleaseTool(), handleCreateRelease)
}

func pushTool() mcp.Tool {
	return mcp.NewTool("push",
		mcp.WithDescription("Push commits to remote. Enforces branch protection."),
		mcp.WithString("cwd", mcp.Description("Working directory of the git repository.")),
		mcp.WithString("remote", mcp.Description("Remote name. Defaults to origin.")),
		mcp.WithString("branch", mcp.Description("Branch to push. Defaults to current.")),
		mcp.WithBoolean("force", mcp.Description("Force push with lease.")),
	)
}

func createPRTool() mcp.Tool {
	return mcp.NewTool("create_pr",
		mcp.WithDescription("Create a pull/merge request. Auto-detects platform."),
		mcp.WithString("cwd", mcp.Description("Working directory of the git repository.")),
		mcp.WithString("title", mcp.Required(), mcp.Description("PR title.")),
		mcp.WithString("body"),
		mcp.WithString("target_branch", mcp.Description("Target branch. Auto-detected if omitted.")),
		mcp.WithString("source_branch", mcp.Description("Source branch. Defaults to current.")),
		mcp.WithBoolean("draft", mcp.Description("Create as draft PR.")),
	)
}

func mergePRTool() mcp.Tool {
	return mcp.NewTool("merge_pr",
		mcp.WithDescription("Merge a pull/merge request. Auto-detects platform."),
		mcp.WithString("cwd", mcp.Description("Working directory of the git repository.")),
		mcp.WithString("identifier", mcp.Required(), mcp.Description("PR number or MR ID.")),
		mcp.WithBoolean("delete_branch", mcp.Description("Delete source branch after merge.")),
		mcp.WithString("strategy", mcp.Description("Merge strategy: merge, squash, or rebase.")),
	)
}

func prStatusTool() mcp.Tool {
	return mcp.NewTool("pr_status",
		mcp.WithDescription("Get pull/merge request status. Auto-detects platform."),
		mcp.WithString("cwd", mcp.Description("Working directory of the git repository.")),
		mcp.WithString("identifier", mcp.Description("PR number or MR ID.")),
		mcp.WithString("branch", mcp.Description("Branch name to look up PR.")),
	)
}

func handlePush(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	cwd := stringArg(request, "cwd", "")
	remote := stringArg(request, "remote", "origin")
	branch := stringArg(request, "branch", "")
	force := boolArg(request, "force")

	if branch == "" {
		detected, err := platform.CurrentBranch(cwd)
		if err != nil {
			return errorResult("cannot determine current branch: %s", err), nil
		}
		branch = detected
	}

	if platform.IsProtectedBranch(branch) {
		return errorResult("denied: cannot push to protected branch '%s'. Create a feature branch: git checkout -b <type>/<slug>", branch), nil
	}

	plat, err := platform.DetectPlatform(cwd)
	if err != nil {
		return errorResult("platform detection failed: %s — fallback: git push %s %s", err, remote, branch), nil
	}

	result, err := plat.Push(ctx, platform.PushOptions{Remote: remote, Branch: branch, Force: force})
	if err != nil {
		msg := "push failed"
		if result != nil {
			msg = result.Output
		}
		return errorResult("push failed (%s): %s — fallback: git push %s %s", plat.PlatformName(), msg, remote, branch), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("pushed %s to %s/%s (%s)", branch, remote, branch, plat.PlatformName())), nil
}

func handleCreatePR(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	cwd := stringArg(request, "cwd", "")
	title := stringArg(request, "title", "")
	body := stringArg(request, "body", "")
	targetBranch := stringArg(request, "target_branch", "")
	sourceBranch := stringArg(request, "source_branch", "")
	draft := boolArg(request, "draft")

	if title == "" {
		return errorResult("title is required"), nil
	}

	if sourceBranch == "" {
		detected, err := platform.CurrentBranch(cwd)
		if err != nil {
			return errorResult("cannot determine current branch: %s", err), nil
		}
		sourceBranch = detected
	}

	if platform.IsProtectedBranch(sourceBranch) {
		return errorResult("denied: cannot create PR from protected branch '%s'. Work on a feature branch.", sourceBranch), nil
	}

	plat, err := platform.DetectPlatform(cwd)
	if err != nil {
		return errorResult("platform detection failed: %s", err), nil
	}

	result, err := plat.CreatePR(ctx, platform.PRCreateOptions{
		Title:        title,
		Body:         body,
		SourceBranch: sourceBranch,
		TargetBranch: targetBranch,
		Draft:        draft,
	})
	if err != nil {
		msg := "create PR failed"
		if result != nil {
			msg = result.Output
		}
		return errorResult("create PR failed (%s): %s", plat.PlatformName(), msg), nil
	}

	output := fmt.Sprintf("PR created on %s", plat.PlatformName())
	if result.URL != "" {
		output = fmt.Sprintf("PR created on %s: %s", plat.PlatformName(), result.URL)
	}
	return mcp.NewToolResultText(output), nil
}

func handleMergePR(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	cwd := stringArg(request, "cwd", "")
	identifier := stringArg(request, "identifier", "")
	deleteBranch := boolArg(request, "delete_branch")
	strategy := stringArg(request, "strategy", "")

	if identifier == "" {
		return errorResult("identifier is required (PR number or MR ID)"), nil
	}

	plat, err := platform.DetectPlatform(cwd)
	if err != nil {
		return errorResult("platform detection failed: %s", err), nil
	}

	result, err := plat.MergePR(ctx, platform.PRMergeOptions{
		Identifier:    identifier,
		DeleteBranch:  deleteBranch,
		MergeStrategy: strategy,
	})
	if err != nil {
		msg := "merge failed"
		if result != nil {
			msg = result.Output
		}
		return errorResult("merge PR failed (%s): %s", plat.PlatformName(), msg), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("PR %s merged on %s", identifier, plat.PlatformName())), nil
}

func handlePRStatus(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	cwd := stringArg(request, "cwd", "")
	identifier := stringArg(request, "identifier", "")
	branch := stringArg(request, "branch", "")

	if identifier == "" && branch == "" {
		detected, err := platform.CurrentBranch(cwd)
		if err != nil {
			return errorResult("cannot determine current branch: %s", err), nil
		}
		branch = detected
	}

	plat, err := platform.DetectPlatform(cwd)
	if err != nil {
		return errorResult("platform detection failed: %s", err), nil
	}

	info, err := plat.PRStatus(ctx, platform.PRStatusOptions{
		Identifier: identifier,
		Branch:     branch,
	})
	if err != nil {
		return errorResult("pr status failed (%s): %s", plat.PlatformName(), err), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf(
		"PR %s (%s)\nTitle: %s\nState: %s\n%s → %s\nURL: %s",
		info.Identifier, plat.PlatformName(),
		info.Title, info.State,
		info.Source, info.Target,
		info.URL,
	)), nil
}

func stringArg(request mcp.CallToolRequest, key string, defaultValue string) string {
	args, ok := request.Params.Arguments.(map[string]any)
	if !ok {
		return defaultValue
	}
	val, ok := args[key].(string)
	if !ok || val == "" {
		return defaultValue
	}
	return val
}

func boolArg(request mcp.CallToolRequest, key string) bool {
	args, ok := request.Params.Arguments.(map[string]any)
	if !ok {
		return false
	}
	val, _ := args[key].(bool)
	return val
}

func errorResult(format string, args ...any) *mcp.CallToolResult {
	return mcp.NewToolResultError(fmt.Sprintf(format, args...))
}
