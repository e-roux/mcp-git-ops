package platform

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"time"
)

type PushOptions struct {
	Remote string
	Branch string
	Force  bool
}

type PRCreateOptions struct {
	Title        string
	Body         string
	SourceBranch string
	TargetBranch string
	Draft        bool
}

type PRMergeOptions struct {
	Identifier    string
	DeleteBranch  bool
	MergeStrategy string
}

type PRStatusOptions struct {
	Identifier string
	Branch     string
}

type CommandResult struct {
	Success bool
	Output  string
	URL     string
}

type PRInfo struct {
	Identifier string
	Title      string
	State      string
	URL        string
	Source     string
	Target     string
}

type ReleaseCreateOptions struct {
	Tag        string
	Title      string
	Notes      string
	Draft      bool
	Prerelease bool
	Target     string
}

type ReleaseInfo struct {
	Tag   string
	Title string
	URL   string
}

type Platform interface {
	PlatformName() string
	Push(ctx context.Context, opts PushOptions) (*CommandResult, error)
	CreatePR(ctx context.Context, opts PRCreateOptions) (*CommandResult, error)
	MergePR(ctx context.Context, opts PRMergeOptions) (*CommandResult, error)
	PRStatus(ctx context.Context, opts PRStatusOptions) (*PRInfo, error)
	CreateRelease(ctx context.Context, opts ReleaseCreateOptions) (*ReleaseInfo, error)
}

var protectedBranches = parseProtectedBranches()

func parseProtectedBranches() []string {
	env := os.Getenv("PROTECTED_BRANCHES")
	if env == "" {
		return []string{"main", "master"}
	}
	parts := strings.Split(env, ",")
	result := make([]string, 0, len(parts))
	for _, b := range parts {
		trimmed := strings.TrimSpace(b)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func IsProtectedBranch(branch string) bool {
	for _, protected := range protectedBranches {
		if branch == protected {
			return true
		}
	}
	return false
}

func DetectPlatform(dir string) (Platform, error) {
	if override := os.Getenv("GIT_OPS_PLATFORM"); override != "" {
		switch strings.ToLower(override) {
		case "github":
			return &GitHubPlatform{Dir: dir}, nil
		case "gitlab":
			return &GitLabPlatform{Dir: dir}, nil
		case "azuredevops", "azure":
			return &AzureDevOpsPlatform{Dir: dir}, nil
		default:
			return nil, fmt.Errorf("unknown platform override: %s", override)
		}
	}

	remoteURL, err := RunGitCommand(dir, "remote", "get-url", "origin")
	if err != nil {
		return nil, fmt.Errorf("cannot read git remote: %w", err)
	}

	lowered := strings.ToLower(remoteURL)

	if strings.Contains(lowered, "github.com") {
		return &GitHubPlatform{Dir: dir}, nil
	}
	if strings.Contains(lowered, "gitlab") {
		return &GitLabPlatform{Dir: dir}, nil
	}
	if strings.Contains(lowered, "dev.azure.com") || strings.Contains(lowered, "visualstudio.com") {
		return &AzureDevOpsPlatform{Dir: dir}, nil
	}

	host := parseHost(remoteURL)
	if host != "" {
		apiPrefix := getAPIPrefix(remoteURL, host)
		if platformType := probePlatform(apiPrefix); platformType != "" {
			switch platformType {
			case "github":
				return &GitHubPlatform{Dir: dir}, nil
			case "gitlab":
				return &GitLabPlatform{Dir: dir}, nil
			}
		}
	}

	if cliAvailable("glab") {
		if _, err := RunExternalCommand(dir, "glab", "repo", "view"); err == nil {
			return &GitLabPlatform{Dir: dir}, nil
		}
	}
	if cliAvailable("gh") {
		if _, err := RunExternalCommand(dir, "gh", "repo", "view", "--json", "name"); err == nil {
			return &GitHubPlatform{Dir: dir}, nil
		}
	}
	if cliAvailable("az") {
		return &AzureDevOpsPlatform{Dir: dir}, nil
	}

	return nil, fmt.Errorf("cannot detect platform from remote: %s", remoteURL)
}

func CurrentBranch(dir string) (string, error) {
	return RunGitCommand(dir, "rev-parse", "--abbrev-ref", "HEAD")
}

func RunGitCommand(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return strings.TrimSpace(string(out)), fmt.Errorf("%s: %w", strings.TrimSpace(string(out)), err)
	}
	return strings.TrimSpace(string(out)), nil
}

func RunExternalCommand(dir string, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return strings.TrimSpace(string(out)), fmt.Errorf("%s: %w", strings.TrimSpace(string(out)), err)
	}
	return strings.TrimSpace(string(out)), nil
}

func cliAvailable(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func ExtractURL(output string) string {
	for _, line := range strings.Split(output, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "https://") {
			return trimmed
		}
	}
	return ""
}

func BoolFlag(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

func parseHost(remoteURL string) string {
	lowered := strings.ToLower(remoteURL)
	if strings.HasPrefix(lowered, "http://") || strings.HasPrefix(lowered, "https://") {
		u, err := url.Parse(remoteURL)
		if err == nil {
			return u.Host
		}
	}
	var urlStr string
	if strings.HasPrefix(lowered, "ssh://") {
		u, err := url.Parse(remoteURL)
		if err == nil {
			urlStr = u.Host
		}
	} else {
		urlStr = remoteURL
		if strings.Contains(urlStr, "@") {
			parts := strings.SplitN(urlStr, "@", 2)
			urlStr = parts[1]
		}
	}
	if idx := strings.Index(urlStr, "/"); idx != -1 {
		urlStr = urlStr[:idx]
	}
	if idx := strings.Index(urlStr, ":"); idx != -1 {
		urlStr = urlStr[:idx]
	}
	return urlStr
}

func getAPIPrefix(remoteURL string, host string) string {
	if strings.HasPrefix(strings.ToLower(remoteURL), "http://") {
		return "http://" + host
	}
	return "https://" + host
}

func probePlatform(apiPrefix string) string {
	client := &http.Client{
		Timeout: 1500 * time.Millisecond,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	ch := make(chan string, 2)

	go func() {
		req, err := http.NewRequest("HEAD", apiPrefix+"/api/v3", nil)
		if err != nil {
			ch <- ""
			return
		}
		resp, err := client.Do(req)
		if err == nil && resp != nil {
			defer resp.Body.Close()
			for k := range resp.Header {
				if strings.HasPrefix(strings.ToLower(k), "x-github-") {
					ch <- "github"
					return
				}
			}
		}
		ch <- ""
	}()

	go func() {
		req, err := http.NewRequest("HEAD", apiPrefix+"/api/v4/", nil)
		if err != nil {
			ch <- ""
			return
		}
		resp, err := client.Do(req)
		if err == nil && resp != nil {
			defer resp.Body.Close()
			for k := range resp.Header {
				if strings.HasPrefix(strings.ToLower(k), "x-gitlab-") {
					ch <- "gitlab"
					return
				}
			}
		}
		ch <- ""
	}()

	for i := 0; i < 2; i++ {
		res := <-ch
		if res != "" {
			return res
		}
	}
	return ""
}
