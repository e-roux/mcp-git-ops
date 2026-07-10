package platform

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

type AzureDevOpsPlatform struct {
	Dir string
}

func (a *AzureDevOpsPlatform) PlatformName() string { return "azuredevops" }

func (a *AzureDevOpsPlatform) Push(_ context.Context, opts PushOptions) (*CommandResult, error) {
	args := []string{"push", opts.Remote, opts.Branch}
	if opts.Force {
		args = []string{"push", "--force-with-lease", opts.Remote, opts.Branch}
	}
	output, err := RunGitCommand(a.Dir, args...)
	if err != nil {
		return &CommandResult{Success: false, Output: output}, err
	}
	return &CommandResult{Success: true, Output: output}, nil
}

func (a *AzureDevOpsPlatform) CreatePR(_ context.Context, opts PRCreateOptions) (*CommandResult, error) {
	args := []string{
		"repos", "pr", "create",
		"--title", opts.Title,
		"--description", opts.Body,
	}
	if opts.TargetBranch != "" {
		args = append(args, "--target-branch", opts.TargetBranch)
	}
	if opts.SourceBranch != "" {
		args = append(args, "--source-branch", opts.SourceBranch)
	}
	if opts.Draft {
		args = append(args, "--draft")
	}

	output, err := RunExternalCommand(a.Dir, "az", args...)
	if err != nil {
		return &CommandResult{Success: false, Output: output}, err
	}

	var prData struct {
		PullRequestID int    `json:"pullRequestId"`
		WebURL        string `json:"url"`
	}
	if err := json.Unmarshal([]byte(output), &prData); err == nil && prData.WebURL != "" {
		return &CommandResult{
			Success: true,
			Output:  fmt.Sprintf("PR #%d created", prData.PullRequestID),
			URL:     prData.WebURL,
		}, nil
	}

	return &CommandResult{Success: true, Output: output, URL: ExtractURL(output)}, nil
}

func (a *AzureDevOpsPlatform) MergePR(_ context.Context, opts PRMergeOptions) (*CommandResult, error) {
	strategy := opts.MergeStrategy
	if strategy == "" {
		strategy = "squash"
	}

	args := []string{
		"repos", "pr", "update",
		"--id", opts.Identifier,
		"--status", "completed",
		"--squash", BoolFlag(strategy == "squash"),
		"--delete-source-branch", BoolFlag(opts.DeleteBranch),
	}

	output, err := RunExternalCommand(a.Dir, "az", args...)
	if err != nil {
		return &CommandResult{Success: false, Output: output}, err
	}
	return &CommandResult{Success: true, Output: output}, nil
}

func (a *AzureDevOpsPlatform) PRStatus(_ context.Context, opts PRStatusOptions) (*PRInfo, error) {
	if opts.Identifier == "" {
		return nil, fmt.Errorf("Azure DevOps requires a PR ID for status lookup")
	}

	args := []string{
		"repos", "pr", "show",
		"--id", opts.Identifier,
		"--output", "json",
	}

	output, err := RunExternalCommand(a.Dir, "az", args...)
	if err != nil {
		return nil, fmt.Errorf("az repos pr show: %s", output)
	}

	var data struct {
		PullRequestID int    `json:"pullRequestId"`
		Title         string `json:"title"`
		Status        string `json:"status"`
		WebURL        string `json:"url"`
		SourceRefName string `json:"sourceRefName"`
		TargetRefName string `json:"targetRefName"`
	}
	if err := json.Unmarshal([]byte(output), &data); err != nil {
		return nil, fmt.Errorf("parse az output: %w", err)
	}

	return &PRInfo{
		Identifier: fmt.Sprintf("%d", data.PullRequestID),
		Title:      data.Title,
		State:      data.Status,
		URL:        data.WebURL,
		Source:     strings.TrimPrefix(data.SourceRefName, "refs/heads/"),
		Target:     strings.TrimPrefix(data.TargetRefName, "refs/heads/"),
	}, nil
}

func (a *AzureDevOpsPlatform) CreateRelease(_ context.Context, opts ReleaseCreateOptions) (*ReleaseInfo, error) {
	return nil, fmt.Errorf("Azure DevOps does not support releases via CLI — push tag %s and create the release in the Azure DevOps portal under Pipelines > Releases", opts.Tag)
}

func (a *AzureDevOpsPlatform) ListReleases(_ context.Context, _ ListReleasesOptions) ([]ReleaseInfo, error) {
	return nil, fmt.Errorf("Azure DevOps does not support releases via CLI")
}
