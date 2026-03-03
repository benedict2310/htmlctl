package cli

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/benedict2310/htmlctl/internal/bundle"
)

var gitCommitRefPattern = regexp.MustCompile(`^[a-fA-F0-9]{7,64}$`)
var gitURLPattern = regexp.MustCompile(`(?i)\b(?:https?|ssh)://[^\s"'<>]+`)

type gitResolvedSource struct {
	SiteDir string
	Source  *bundle.Source
	cleanup func()
}

func (s gitResolvedSource) Cleanup() {
	if s.cleanup != nil {
		s.cleanup()
	}
}

func resolveApplySource(ctx context.Context, from, fromGit, ref, subdir string) (string, *bundle.Source, func(), error) {
	from = strings.TrimSpace(from)
	fromGit = strings.TrimSpace(fromGit)
	ref = strings.TrimSpace(ref)
	subdir = strings.TrimSpace(subdir)

	switch {
	case from != "" && fromGit != "":
		return "", nil, nil, fmt.Errorf("--from and --from-git are mutually exclusive")
	case from == "" && fromGit == "":
		return "", nil, nil, fmt.Errorf("one of --from or --from-git is required")
	case fromGit == "":
		if ref != "" {
			return "", nil, nil, fmt.Errorf("--ref requires --from-git")
		}
		if subdir != "" {
			return "", nil, nil, fmt.Errorf("--subdir requires --from-git")
		}
		return from, nil, func() {}, nil
	case ref == "":
		return "", nil, nil, fmt.Errorf("--ref is required with --from-git")
	}

	resolved, err := resolveGitSource(ctx, fromGit, ref, subdir)
	if err != nil {
		return "", nil, nil, err
	}
	return resolved.SiteDir, resolved.Source, resolved.Cleanup, nil
}

func resolveGitSource(ctx context.Context, repo, ref, subdir string) (gitResolvedSource, error) {
	if _, err := exec.LookPath("git"); err != nil {
		return gitResolvedSource{}, fmt.Errorf("git binary not found; install git to use --from-git")
	}
	if !gitCommitRefPattern.MatchString(strings.TrimSpace(ref)) {
		return gitResolvedSource{}, fmt.Errorf("--ref must be a pinned commit SHA, not a branch or symbolic ref")
	}

	workDir, err := os.MkdirTemp("", "htmlctl-git-*")
	if err != nil {
		return gitResolvedSource{}, fmt.Errorf("create temporary git workdir: %w", err)
	}
	cleanup := func() { _ = os.RemoveAll(workDir) }

	if err := runGit(ctx, "", "clone", "--no-checkout", repo, workDir); err != nil {
		cleanup()
		return gitResolvedSource{}, err
	}
	if err := ensurePinnedCommitRef(ctx, workDir, ref); err != nil {
		cleanup()
		return gitResolvedSource{}, err
	}
	if err := runGit(ctx, workDir, "checkout", "--detach", ref); err != nil {
		cleanup()
		return gitResolvedSource{}, err
	}

	resolvedRef, err := gitOutput(ctx, workDir, "rev-parse", "HEAD")
	if err != nil {
		cleanup()
		return gitResolvedSource{}, err
	}
	if !strings.HasPrefix(strings.ToLower(strings.TrimSpace(resolvedRef)), strings.ToLower(strings.TrimSpace(ref))) {
		cleanup()
		return gitResolvedSource{}, fmt.Errorf("--ref must be a pinned commit SHA, not a branch or symbolic ref")
	}

	siteDir := workDir
	if subdir != "" {
		cleanSubdir, err := validateGitSubdir(subdir)
		if err != nil {
			cleanup()
			return gitResolvedSource{}, err
		}
		siteDir = filepath.Join(workDir, filepath.FromSlash(cleanSubdir))
	}
	info, err := os.Stat(siteDir)
	if err != nil {
		cleanup()
		if os.IsNotExist(err) {
			return gitResolvedSource{}, fmt.Errorf("git source subdir %q not found", subdir)
		}
		return gitResolvedSource{}, fmt.Errorf("stat git source directory: %w", err)
	}
	if !info.IsDir() {
		cleanup()
		return gitResolvedSource{}, fmt.Errorf("git source path %q is not a directory", subdir)
	}
	if err := ensureGitSourceNoSymlinks(siteDir); err != nil {
		cleanup()
		return gitResolvedSource{}, err
	}

	return gitResolvedSource{
		SiteDir: siteDir,
		Source: &bundle.Source{
			Type:   "git",
			Repo:   redactGitRepo(repo),
			Ref:    strings.TrimSpace(resolvedRef),
			Subdir: normalizeOptionalSubdir(subdir),
		},
		cleanup: cleanup,
	}, nil
}

func validateGitSubdir(subdir string) (string, error) {
	clean := filepath.ToSlash(filepath.Clean(strings.TrimSpace(subdir)))
	if clean == "." || clean == "" {
		return "", fmt.Errorf("git source subdir is empty")
	}
	if strings.HasPrefix(clean, "/") || clean == ".." || strings.HasPrefix(clean, "../") || strings.Contains(clean, "/../") {
		return "", fmt.Errorf("git source subdir must stay within the repository")
	}
	return clean, nil
}

func normalizeOptionalSubdir(subdir string) string {
	if strings.TrimSpace(subdir) == "" {
		return ""
	}
	clean, err := validateGitSubdir(subdir)
	if err != nil {
		return ""
	}
	return clean
}

func ensureGitSourceNoSymlinks(root string) error {
	info, err := os.Lstat(root)
	if err != nil {
		return fmt.Errorf("lstat git source root: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("git source must not contain symlinks: %s", filepath.Base(root))
	}
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == root {
			return nil
		}
		if d.Type()&os.ModeSymlink == 0 {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return fmt.Errorf("rel git source symlink: %w", err)
		}
		return fmt.Errorf("git source must not contain symlinks: %s", filepath.ToSlash(rel))
	})
}

func runGit(ctx context.Context, dir string, args ...string) error {
	_, err := gitOutput(ctx, dir, args...)
	return err
}

func ensurePinnedCommitRef(ctx context.Context, dir, ref string) error {
	for _, refName := range []string{
		"refs/heads/" + ref,
		"refs/tags/" + ref,
		"refs/remotes/origin/" + ref,
	} {
		exists, err := gitRefExists(ctx, dir, refName)
		if err != nil {
			return err
		}
		if exists {
			return fmt.Errorf("--ref must be a pinned commit SHA, not a branch or symbolic ref")
		}
	}
	return nil
}

func gitRefExists(ctx context.Context, dir, refName string) (bool, error) {
	cmd := exec.CommandContext(ctx, "git", "show-ref", "--verify", "--quiet", refName)
	if strings.TrimSpace(dir) != "" {
		cmd.Dir = dir
	}
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
			return false, nil
		}
		return false, fmt.Errorf("git show-ref --verify --quiet %s failed: %w", refName, err)
	}
	return true, nil
}

func gitOutput(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	if strings.TrimSpace(dir) != "" {
		cmd.Dir = dir
	}
	out, err := cmd.CombinedOutput()
	redactedArgs := redactGitArgs(args)
	if err == nil {
		return strings.TrimSpace(string(out)), nil
	}
	msg := strings.TrimSpace(string(out))
	if msg == "" {
		return "", fmt.Errorf("git %s failed: %w", strings.Join(redactedArgs, " "), err)
	}
	return "", fmt.Errorf("git %s failed: %s", strings.Join(redactedArgs, " "), redactGitText(msg))
}

func redactGitText(v string) string {
	v = gitURLPattern.ReplaceAllStringFunc(v, func(match string) string {
		if redacted, changed := redactGitToken(match); changed {
			return redacted
		}
		return match
	})
	return v
}

func redactGitRepo(repo string) string {
	if redacted, changed := redactGitToken(strings.TrimSpace(repo)); changed {
		return redacted
	}
	return strings.TrimSpace(repo)
}

func redactGitArgs(args []string) []string {
	out := make([]string, 0, len(args))
	for _, arg := range args {
		if redacted, changed := redactGitToken(arg); changed {
			out = append(out, redacted)
			continue
		}
		out = append(out, arg)
	}
	return out
}

func redactGitToken(token string) (string, bool) {
	if parsed, err := url.Parse(token); err == nil && parsed.Scheme != "" {
		if parsed.User != nil {
			if parsed.Scheme == "ssh" {
				if _, hasPassword := parsed.User.Password(); !hasPassword {
					return token, false
				}
			}
			parsed.User = nil
			return parsed.String(), true
		}
		return token, false
	}
	return token, false
}
