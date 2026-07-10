package platform

import (
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"testing"
)

func TestParseHost(t *testing.T) {
	cases := []struct {
		url      string
		expected string
	}{
		{"https://github.com/e-roux/mcp-git-ops.git", "github.com"},
		{"http://github.com/e-roux/mcp-git-ops", "github.com"},
		{"git@github.com:e-roux/mcp-git-ops.git", "github.com"},
		{"ssh://git@gitlab.company.com:22/group/project.git", "gitlab.company.com"},
		{"git@github.company.com:owner/repo.git", "github.company.com"},
	}

	for _, tc := range cases {
		t.Run(tc.url, func(t *testing.T) {
			got := parseHost(tc.url)
			if got != tc.expected {
				t.Errorf("parseHost(%q) = %q, want %q", tc.url, got, tc.expected)
			}
		})
	}
}

func TestDetectPlatformViaProbing(t *testing.T) {
	ghServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v3" {
			w.Header().Set("X-GitHub-Request-Id", "123")
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ghServer.Close()

	glServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v4/" {
			w.Header().Set("X-Gitlab-Meta", "some-value")
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer glServer.Close()

	tempDir, err := os.MkdirTemp("", "mcp-git-ops-probe-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	runCmd := func(dir string, name string, args ...string) {
		cmd := exec.Command(name, args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("command failed: %s %v: %v\nOutput: %s", name, args, err, string(out))
		}
	}

	runCmd(tempDir, "git", "init")
	runCmd(tempDir, "git", "config", "user.name", "Test User")
	runCmd(tempDir, "git", "config", "user.email", "test@example.com")

	cases := []struct {
		name         string
		remoteURL    string
		expectedName string
	}{
		{
			name:         "Mock generic GitHub Enterprise (detected via /api/v3)",
			remoteURL:    ghServer.URL + "/generic-ghe/repo.git",
			expectedName: "github",
		},
		{
			name:         "Mock generic GitLab (detected via /api/v4)",
			remoteURL:    glServer.URL + "/generic-gitlab/repo.git",
			expectedName: "gitlab",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_ = exec.Command("git", "-C", tempDir, "remote", "remove", "origin").Run()
			runCmd(tempDir, "git", "remote", "add", "origin", tc.remoteURL)

			plat, err := DetectPlatform(tempDir)
			if err != nil {
				t.Fatalf("DetectPlatform failed: %v", err)
			}

			if plat.PlatformName() != tc.expectedName {
				t.Errorf("DetectPlatform(%q) = %q, want %q", tc.remoteURL, plat.PlatformName(), tc.expectedName)
			}
		})
	}
}

func TestDetectPlatform(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "mcp-git-ops-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	runCmd := func(dir string, name string, args ...string) {
		cmd := exec.Command(name, args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("command failed: %s %v: %v\nOutput: %s", name, args, err, string(out))
		}
	}

	runCmd(tempDir, "git", "init")
	runCmd(tempDir, "git", "config", "user.name", "Test User")
	runCmd(tempDir, "git", "config", "user.email", "test@example.com")

	cases := []struct {
		name         string
		remoteURL    string
		expectedName string
	}{
		{
			name:         "GitHub public URL",
			remoteURL:    "https://github.com/e-roux/mcp-git-ops.git",
			expectedName: "github",
		},
		{
			name:         "GitHub public SSH URL",
			remoteURL:    "git@github.com:e-roux/mcp-git-ops.git",
			expectedName: "github",
		},
		{
			name:         "GitLab public URL",
			remoteURL:    "https://gitlab.com/group/project.git",
			expectedName: "gitlab",
		},
		{
			name:         "Azure DevOps",
			remoteURL:    "https://dev.azure.com/org/project/_git/repo",
			expectedName: "azuredevops",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_ = exec.Command("git", "-C", tempDir, "remote", "remove", "origin").Run()

			runCmd(tempDir, "git", "remote", "add", "origin", tc.remoteURL)

			plat, err := DetectPlatform(tempDir)
			if err != nil {
				t.Fatalf("DetectPlatform failed: %v", err)
			}

			if plat.PlatformName() != tc.expectedName {
				t.Errorf("DetectPlatform(%q) = %q, want %q", tc.remoteURL, plat.PlatformName(), tc.expectedName)
			}
		})
	}
}
