package cli

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveGitSourceLocalRepo(t *testing.T) {
	repoDir, head := createGitSiteRepo(t, "")

	resolved, err := resolveGitSource(context.Background(), repoDir, head, "")
	if err != nil {
		t.Fatalf("resolveGitSource() error = %v", err)
	}
	defer resolved.Cleanup()

	if resolved.Source == nil || resolved.Source.Type != "git" || len(resolved.Source.Ref) != 40 {
		t.Fatalf("unexpected source metadata %#v", resolved.Source)
	}
	if _, err := os.Stat(filepath.Join(resolved.SiteDir, "website.yaml")); err != nil {
		t.Fatalf("expected checked out site root: %v", err)
	}
}

func TestResolveGitSourceSubdir(t *testing.T) {
	repoDir, head := createGitSiteRepo(t, "site")

	resolved, err := resolveGitSource(context.Background(), repoDir, head, "site")
	if err != nil {
		t.Fatalf("resolveGitSource() error = %v", err)
	}
	defer resolved.Cleanup()

	if got := filepath.Base(resolved.SiteDir); got != "site" {
		t.Fatalf("expected subdir checkout root, got %q", resolved.SiteDir)
	}
	if _, err := os.Stat(filepath.Join(resolved.SiteDir, "website.yaml")); err != nil {
		t.Fatalf("expected checked out subdir site root: %v", err)
	}
}

func TestResolveGitSourceMissingRefFails(t *testing.T) {
	repoDir, _ := createGitSiteRepo(t, "")

	_, err := resolveGitSource(context.Background(), repoDir, "missing-ref", "")
	if err == nil {
		t.Fatalf("expected missing ref error")
	}
	if !strings.Contains(err.Error(), "pinned commit SHA") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestResolveGitSourceRejectsBranchName(t *testing.T) {
	repoDir, _ := createGitSiteRepo(t, "")

	_, err := resolveGitSource(context.Background(), repoDir, "main", "")
	if err == nil {
		t.Fatalf("expected branch name rejection")
	}
	if !strings.Contains(err.Error(), "pinned commit SHA") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestResolveGitSourceRejectsHexBranchName(t *testing.T) {
	repoDir, _ := createGitSiteRepo(t, "")
	runGitTest(t, repoDir, "branch", "deadbeef")

	_, err := resolveGitSource(context.Background(), repoDir, "deadbeef", "")
	if err == nil {
		t.Fatalf("expected hex branch name rejection")
	}
	if !strings.Contains(err.Error(), "pinned commit SHA") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestResolveGitSourceRejectsSymlinkedContent(t *testing.T) {
	repoDir, head := createGitSiteRepo(t, "")
	target := filepath.Join(t.TempDir(), "outside.html")
	if err := os.WriteFile(target, []byte("<p>outside</p>"), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", target, err)
	}
	linkPath := filepath.Join(repoDir, "components", "header.html")
	if err := os.Remove(linkPath); err != nil {
		t.Fatalf("Remove(%q) error = %v", linkPath, err)
	}
	if err := os.Symlink(target, linkPath); err != nil {
		t.Fatalf("Symlink(%q, %q) error = %v", target, linkPath, err)
	}
	runGitTest(t, repoDir, "add", "components/header.html")
	runGitTest(t, repoDir, "commit", "-m", "symlink hero")
	head = strings.TrimSpace(runGitOutputTest(t, repoDir, "rev-parse", "HEAD"))

	_, err := resolveGitSource(context.Background(), repoDir, head, "")
	if err == nil {
		t.Fatalf("expected symlink rejection")
	}
	if !strings.Contains(err.Error(), "must not contain symlinks") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestResolveApplySourceValidation(t *testing.T) {
	if _, _, _, err := resolveApplySource(context.Background(), "", "", "", ""); err == nil {
		t.Fatalf("expected missing source mode error")
	}
	if _, _, _, err := resolveApplySource(context.Background(), "./site", "./repo", "abc", ""); err == nil {
		t.Fatalf("expected mutually exclusive source mode error")
	}
	if _, _, _, err := resolveApplySource(context.Background(), "./site", "", "abc", ""); err == nil {
		t.Fatalf("expected --ref without --from-git error")
	}
	if _, _, _, err := resolveApplySource(context.Background(), "", "./repo", "", ""); err == nil {
		t.Fatalf("expected missing --ref error")
	}
}

func TestRedactGitTextRemovesCredentials(t *testing.T) {
	redacted := redactGitText("fatal: could not read from https://user:secret@example.com/org/repo.git")
	if strings.Contains(redacted, "secret") {
		t.Fatalf("expected credentials to be redacted, got %q", redacted)
	}
}

func TestRedactGitTextRemovesQuotedCredentials(t *testing.T) {
	redacted := redactGitText("fatal: could not read from 'https://user:secret@example.com/org/repo.git'")
	if strings.Contains(redacted, "secret") {
		t.Fatalf("expected quoted credentials to be redacted, got %q", redacted)
	}
}

func TestRedactGitArgsRemovesCredentials(t *testing.T) {
	args := redactGitArgs([]string{"clone", "https://user:secret@example.com/org/repo.git", "/tmp/repo"})
	if strings.Contains(strings.Join(args, " "), "secret") {
		t.Fatalf("expected args to be redacted, got %#v", args)
	}
}

func TestRedactGitRepoPreservesSCPStyleRemote(t *testing.T) {
	repo := "git@github.com:org/repo.git"
	if got := redactGitRepo(repo); got != repo {
		t.Fatalf("expected scp-style repo to be preserved, got %q", got)
	}
}

func createGitSiteRepo(t *testing.T, subdir string) (string, string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}

	repoDir := t.TempDir()
	siteDir := repoDir
	if strings.TrimSpace(subdir) != "" {
		siteDir = filepath.Join(repoDir, subdir)
		if err := os.MkdirAll(siteDir, 0o755); err != nil {
			t.Fatalf("MkdirAll(%q) error = %v", siteDir, err)
		}
	}
	copyTree(t, writeApplySiteFixture(t), siteDir)

	runGitTest(t, repoDir, "init")
	runGitTest(t, repoDir, "config", "user.name", "Test User")
	runGitTest(t, repoDir, "config", "user.email", "test@example.com")
	runGitTest(t, repoDir, "add", ".")
	runGitTest(t, repoDir, "commit", "-m", "initial")
	head := strings.TrimSpace(runGitOutputTest(t, repoDir, "rev-parse", "HEAD"))
	return repoDir, head
}

func copyTree(t *testing.T, src, dst string) {
	t.Helper()
	if err := filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	}); err != nil {
		t.Fatalf("copyTree(%q, %q) error = %v", src, dst, err)
	}
}

func runGitTest(t *testing.T, dir string, args ...string) {
	t.Helper()
	if err := runGit(context.Background(), dir, args...); err != nil {
		t.Fatalf("runGit(%v) error = %v", args, err)
	}
}

func runGitOutputTest(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v error = %v output=%s", args, err, string(out))
	}
	return string(out)
}
