package gitops

import (
	"context"
	"fmt"
	"testing"

	"github.com/e-roux/mcp-git-ops/internal/platform"
)

type mockPlatform struct {
	name          string
	pushResult    *platform.CommandResult
	pushErr       error
	createResult  *platform.CommandResult
	createErr     error
	mergeResult   *platform.CommandResult
	mergeErr      error
	statusResult  *platform.PRInfo
	statusErr     error
	releaseResult *platform.ReleaseInfo
	releaseErr    error
}

func (m *mockPlatform) PlatformName() string { return m.name }

func (m *mockPlatform) Push(_ context.Context, _ platform.PushOptions) (*platform.CommandResult, error) {
	return m.pushResult, m.pushErr
}

func (m *mockPlatform) CreatePR(_ context.Context, _ platform.PRCreateOptions) (*platform.CommandResult, error) {
	return m.createResult, m.createErr
}

func (m *mockPlatform) MergePR(_ context.Context, _ platform.PRMergeOptions) (*platform.CommandResult, error) {
	return m.mergeResult, m.mergeErr
}

func (m *mockPlatform) PRStatus(_ context.Context, _ platform.PRStatusOptions) (*platform.PRInfo, error) {
	return m.statusResult, m.statusErr
}

func (m *mockPlatform) CreateRelease(_ context.Context, _ platform.ReleaseCreateOptions) (*platform.ReleaseInfo, error) {
	return m.releaseResult, m.releaseErr
}

func TestAllPlatformsSatisfyInterface(t *testing.T) {
	var _ platform.Platform = &platform.GitHubPlatform{}
	var _ platform.Platform = &platform.GitLabPlatform{}
	var _ platform.Platform = &platform.AzureDevOpsPlatform{}
	var _ platform.Platform = &mockPlatform{}
}

func TestIsProtectedBranch(t *testing.T) {
	cases := []struct {
		branch   string
		expected bool
	}{
		{"main", true},
		{"master", true},
		{"feat/new-feature", false},
		{"fix/bug-123", false},
		{"develop", false},
		{"release/1.0", false},
	}

	for _, tc := range cases {
		t.Run(tc.branch, func(t *testing.T) {
			got := platform.IsProtectedBranch(tc.branch)
			if got != tc.expected {
				t.Errorf("IsProtectedBranch(%q) = %v, want %v", tc.branch, got, tc.expected)
			}
		})
	}
}

func TestExtractURL(t *testing.T) {
	cases := []struct {
		name     string
		input    string
		expected string
	}{
		{
			"github pr output",
			"Creating pull request...\nhttps://github.com/owner/repo/pull/42\n",
			"https://github.com/owner/repo/pull/42",
		},
		{
			"no url in output",
			"some output without urls",
			"",
		},
		{
			"multiple lines with url",
			"info: something\nhttps://gitlab.com/group/project/-/merge_requests/7\ndone",
			"https://gitlab.com/group/project/-/merge_requests/7",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := platform.ExtractURL(tc.input)
			if got != tc.expected {
				t.Errorf("ExtractURL() = %q, want %q", got, tc.expected)
			}
		})
	}
}

func TestBoolFlag(t *testing.T) {
	if platform.BoolFlag(true) != "true" {
		t.Error("BoolFlag(true) should return \"true\"")
	}
	if platform.BoolFlag(false) != "false" {
		t.Error("BoolFlag(false) should return \"false\"")
	}
}

func TestParseGitLabMROutput(t *testing.T) {
	output := `title: Fix login redirect
state: merged
url: https://gitlab.com/group/project/-/merge_requests/42
`
	info := platform.ParseGitLabMROutput(output)

	if info.Title != "Fix login redirect" {
		t.Errorf("Title = %q, want %q", info.Title, "Fix login redirect")
	}
	if info.State != "merged" {
		t.Errorf("State = %q, want %q", info.State, "merged")
	}
	if info.URL != "https://gitlab.com/group/project/-/merge_requests/42" {
		t.Errorf("URL = %q", info.URL)
	}
}

func TestToolDefinitions(t *testing.T) {
	tools := []struct {
		name string
		tool func() interface{ GetName() string }
	}{
		{"push", func() interface{ GetName() string } { t := pushTool(); return &t }},
		{"create_pr", func() interface{ GetName() string } { t := createPRTool(); return &t }},
		{"merge_pr", func() interface{ GetName() string } { t := mergePRTool(); return &t }},
		{"pr_status", func() interface{ GetName() string } { t := prStatusTool(); return &t }},
		{"release_status", func() interface{ GetName() string } { t := releaseStatusTool(); return &t }},
		{"create_release", func() interface{ GetName() string } { t := createReleaseTool(); return &t }},
	}

	for _, tc := range tools {
		t.Run(tc.name, func(t *testing.T) {
			tool := tc.tool()
			if tool.GetName() != tc.name {
				t.Errorf("tool name = %q, want %q", tool.GetName(), tc.name)
			}
		})
	}
}

func TestHandlePushProtectedBranch(t *testing.T) {
	for _, branch := range []string{"main", "master"} {
		if !platform.IsProtectedBranch(branch) {
			t.Errorf("expected %q to be protected", branch)
		}
	}
}

func TestMockPlatformPush(t *testing.T) {
	mock := &mockPlatform{
		name:       "test",
		pushResult: &platform.CommandResult{Success: true, Output: "pushed"},
	}

	result, err := mock.Push(context.Background(), platform.PushOptions{
		Remote: "origin",
		Branch: "feat/test",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Error("expected success")
	}
}

func TestMockPlatformCreatePR(t *testing.T) {
	mock := &mockPlatform{
		name:         "test",
		createResult: &platform.CommandResult{Success: true, URL: "https://example.com/pr/1"},
	}

	result, err := mock.CreatePR(context.Background(), platform.PRCreateOptions{
		Title: "test PR",
		Body:  "test body",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.URL != "https://example.com/pr/1" {
		t.Errorf("URL = %q", result.URL)
	}
}

func TestMockPlatformMergePR(t *testing.T) {
	mock := &mockPlatform{
		name:        "test",
		mergeResult: &platform.CommandResult{Success: true, Output: "merged"},
	}

	result, err := mock.MergePR(context.Background(), platform.PRMergeOptions{
		Identifier: "42",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Error("expected success")
	}
}

func TestMockPlatformPRStatus(t *testing.T) {
	mock := &mockPlatform{
		name: "test",
		statusResult: &platform.PRInfo{
			Identifier: "42",
			Title:      "test PR",
			State:      "open",
			URL:        "https://example.com/pr/42",
			Source:     "feat/test",
			Target:     "main",
		},
	}

	info, err := mock.PRStatus(context.Background(), platform.PRStatusOptions{Identifier: "42"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.Identifier != "42" {
		t.Errorf("Identifier = %q", info.Identifier)
	}
	if info.State != "open" {
		t.Errorf("State = %q", info.State)
	}
}

func TestMockPlatformFailure(t *testing.T) {
	mock := &mockPlatform{
		name:       "test",
		pushResult: &platform.CommandResult{Success: false, Output: "rejected"},
		pushErr:    fmt.Errorf("push rejected"),
	}

	result, err := mock.Push(context.Background(), platform.PushOptions{
		Remote: "origin",
		Branch: "feat/test",
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if result.Success {
		t.Error("expected failure")
	}
}

func TestMockPlatformCreateRelease(t *testing.T) {
	mock := &mockPlatform{
		name:          "test",
		releaseResult: &platform.ReleaseInfo{Tag: "v1.0.0", Title: "v1.0.0", URL: "https://example.com/releases/v1.0.0"},
	}

	info, err := mock.CreateRelease(context.Background(), platform.ReleaseCreateOptions{
		Tag:   "v1.0.0",
		Title: "v1.0.0",
		Notes: "## Fixed\n- **scope**: description",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.URL != "https://example.com/releases/v1.0.0" {
		t.Errorf("URL = %q", info.URL)
	}
}

func TestInferNextVersion(t *testing.T) {
	cases := []struct {
		latestTag string
		sections  []string
		wantBump  string
	}{
		{"v1.2.3", []string{"Fixed"}, "v1.2.4"},
		{"v1.2.3", []string{"Added"}, "v1.3.0"},
		{"v1.2.3", []string{"Changed"}, "v1.3.0"},
		{"v1.2.3", []string{"Removed"}, "v2.0.0"},
		{"v1.2.3", []string{"Added", "Fixed"}, "v1.3.0"},
		{"v1.2.3", []string{"Security"}, "v1.2.4"},
	}
	for _, tc := range cases {
		t.Run(tc.latestTag+"/"+tc.sections[0], func(t *testing.T) {
			got := InferNextVersion(tc.latestTag, tc.sections)
			if got != tc.wantBump {
				t.Errorf("InferNextVersion(%q, %v) = %q, want %q", tc.latestTag, tc.sections, got, tc.wantBump)
			}
		})
	}
}
