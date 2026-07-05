package platform

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

type GitHubPlatform struct {
	Dir string
}

func (g *GitHubPlatform) PlatformName() string { return "github" }

func (g *GitHubPlatform) Push(_ context.Context, opts PushOptions) (*CommandResult, error) {
	args := []string{"push", opts.Remote, opts.Branch}
	if opts.Force {
		args = []string{"push", "--force-with-lease", opts.Remote, opts.Branch}
	}
	output, err := RunGitCommand(g.Dir, args...)
	if err != nil {
		return &CommandResult{Success: false, Output: output}, err
	}
	return &CommandResult{Success: true, Output: output}, nil
}

func (g *GitHubPlatform) CreatePR(_ context.Context, opts PRCreateOptions) (*CommandResult, error) {
	args := []string{"pr", "create", "--title", opts.Title, "--body", opts.Body}
	if opts.TargetBranch != "" {
		args = append(args, "--base", opts.TargetBranch)
	}
	if opts.SourceBranch != "" {
		args = append(args, "--head", opts.SourceBranch)
	}
	if opts.Draft {
		args = append(args, "--draft")
	}

	output, err := RunExternalCommand(g.Dir, "gh", args...)
	if err != nil {
		return &CommandResult{Success: false, Output: output}, err
	}
	return &CommandResult{Success: true, Output: output, URL: ExtractURL(output)}, nil
}

func (g *GitHubPlatform) MergePR(_ context.Context, opts PRMergeOptions) (*CommandResult, error) {
	args := []string{"pr", "merge", opts.Identifier}

	switch strings.ToLower(opts.MergeStrategy) {
	case "squash":
		args = append(args, "--squash")
	case "rebase":
		args = append(args, "--rebase")
	default:
		args = append(args, "--merge")
	}
	if opts.DeleteBranch {
		args = append(args, "--delete-branch")
	}

	output, err := RunExternalCommand(g.Dir, "gh", args...)
	if err != nil {
		return &CommandResult{Success: false, Output: output}, err
	}
	return &CommandResult{Success: true, Output: output}, nil
}

func (g *GitHubPlatform) PRStatus(_ context.Context, opts PRStatusOptions) (*PRInfo, error) {
	ref := opts.Identifier
	if ref == "" && opts.Branch != "" {
		ref = opts.Branch
	}

	args := []string{"pr", "view"}
	if ref != "" {
		args = append(args, ref)
	}
	args = append(args, "--json", "number,title,state,url,headRefName,baseRefName")

	output, err := RunExternalCommand(g.Dir, "gh", args...)
	if err != nil {
		return nil, fmt.Errorf("gh pr view: %s", output)
	}

	var data struct {
		Number      int    `json:"number"`
		Title       string `json:"title"`
		State       string `json:"state"`
		URL         string `json:"url"`
		HeadRefName string `json:"headRefName"`
		BaseRefName string `json:"baseRefName"`
	}
	if err := json.Unmarshal([]byte(output), &data); err != nil {
		return nil, fmt.Errorf("parse gh output: %w", err)
	}

	return &PRInfo{
		Identifier: fmt.Sprintf("%d", data.Number),
		Title:      data.Title,
		State:      data.State,
		URL:        data.URL,
		Source:     data.HeadRefName,
		Target:     data.BaseRefName,
	}, nil
}

func (g *GitHubPlatform) CreateRelease(_ context.Context, opts ReleaseCreateOptions) (*ReleaseInfo, error) {
	args := []string{"release", "create", opts.Tag, "--title", opts.Title, "--notes", opts.Notes}
	if opts.Draft {
		args = append(args, "--draft")
	}
	if opts.Prerelease {
		args = append(args, "--prerelease")
	}
	if opts.Target != "" {
		args = append(args, "--target", opts.Target)
	}

	output, err := RunExternalCommand(g.Dir, "gh", args...)
	if err != nil {
		return nil, fmt.Errorf("gh release create: %s", output)
	}
	return &ReleaseInfo{Tag: opts.Tag, Title: opts.Title, URL: ExtractURL(output)}, nil
}
