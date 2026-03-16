package cli

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/benedict2310/htmlctl/internal/names"
	"github.com/benedict2310/htmlctl/internal/output"
	"github.com/spf13/cobra"
)

type siteExplainReport struct {
	Skeleton            []string `json:"skeleton" yaml:"skeleton"`
	AuthoringModel      []string `json:"authoringModel" yaml:"authoringModel"`
	ComponentRules      []string `json:"componentRules" yaml:"componentRules"`
	SupportedApplyPaths []string `json:"supportedApplyPaths" yaml:"supportedApplyPaths"`
	GeneratedArtifacts  []string `json:"generatedArtifacts" yaml:"generatedArtifacts"`
}

const exportMarkerFile = ".htmlctl-export"

func newSiteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "site",
		Short: "Explain, scaffold, and export htmlctl site layouts",
	}

	cmd.AddCommand(newSiteExplainCmd())
	cmd.AddCommand(newSiteInitCmd())
	cmd.AddCommand(newSiteExportCmd())
	return cmd
}

func newSiteExplainCmd() *cobra.Command {
	var outputMode string
	cmd := &cobra.Command{
		Use:   "explain",
		Short: "Explain the supported htmlctl site model and file layout",
		RunE: func(cmd *cobra.Command, args []string) error {
			report := siteExplainReport{
				Skeleton: []string{
					"website.yaml",
					"pages/*.page.yaml",
					"components/*.html",
					"optional components/<name>.css",
					"optional components/<name>.js",
					"styles/tokens.css",
					"styles/default.css",
					"optional scripts/site.js",
					"optional assets/**",
					"optional branding/**",
				},
				AuthoringModel: []string{
					"Pages compose ordered layout includes; there is no separate inline page-body file type.",
					"Use reusable section components for shared blocks and page-body components for route-specific content.",
					"Website-level metadata belongs in website.yaml and branding/ rather than individual page files.",
				},
				ComponentRules: []string{
					"Exactly one root element per component.",
					"No script tags or inline on* handlers in component HTML.",
					"Root tag must be one of section, header, footer, main, nav, article, div.",
					"Anchor-navigable components should use id=<componentName> on the root element.",
				},
				SupportedApplyPaths: []string{
					"site/ directory root",
					"website.yaml",
					"pages/*.page.yaml",
					"components/<name>.html",
					"components/<name>.css",
					"components/<name>.js",
					"styles/tokens.css",
					"styles/default.css",
					"scripts/site.js",
					"assets/**",
					"branding/**",
				},
				GeneratedArtifacts: []string{
					"OG images are generated at build time when canonicalURL is absolute and image fields are empty.",
					"website.yaml controls favicon, robots.txt, sitemap.xml, llms.txt, and structured data generation.",
					"promote copies release artifacts byte-for-byte; it does not rebuild generated metadata for the target environment.",
				},
			}
			format, err := output.ParseFormat(outputMode)
			if err != nil {
				return err
			}
			if format != output.FormatTable {
				return output.WriteStructured(cmd.OutOrStdout(), format, report)
			}
			writeSection := func(title string, lines []string) {
				fmt.Fprintf(cmd.OutOrStdout(), "%s:\n", title)
				for _, line := range lines {
					fmt.Fprintf(cmd.OutOrStdout(), "- %s\n", line)
				}
				fmt.Fprintln(cmd.OutOrStdout())
			}
			writeSection("Skeleton", report.Skeleton)
			writeSection("Authoring Model", report.AuthoringModel)
			writeSection("Component Rules", report.ComponentRules)
			writeSection("Supported Apply Paths", report.SupportedApplyPaths)
			writeSection("Generated Artifacts", report.GeneratedArtifacts)
			return nil
		},
	}
	cmd.Flags().StringVarP(&outputMode, "output", "o", "table", "Output format (table|json|yaml)")
	return cmd
}

func newSiteInitCmd() *cobra.Command {
	var templateName string
	var force bool
	cmd := &cobra.Command{
		Use:   "init <dir>",
		Short: "Create a minimal valid htmlctl site skeleton",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(templateName) != "" && strings.TrimSpace(templateName) != "minimal" {
				return fmt.Errorf("unsupported template %q (supported: minimal)", templateName)
			}
			target := strings.TrimSpace(args[0])
			if target == "" {
				return fmt.Errorf("target directory is required")
			}
			if err := ensureInitTarget(target, force); err != nil {
				return err
			}
			websiteName := deriveSiteName(target)
			title := displayTitleForSite(websiteName)
			files := map[string]string{
				"website.yaml": fmt.Sprintf("apiVersion: htmlctl.dev/v1\nkind: Website\nmetadata:\n  name: %s\nspec:\n  defaultStyleBundle: default\n  baseTemplate: default\n", websiteName),
				filepath.ToSlash(filepath.Join("pages", "index.page.yaml")): fmt.Sprintf("apiVersion: htmlctl.dev/v1\nkind: Page\nmetadata:\n  name: index\nspec:\n  route: /\n  title: %s\n  description: %s landing page\n  layout:\n    - include: hero\n", title, title),
				filepath.ToSlash(filepath.Join("components", "hero.html")):  fmt.Sprintf("<section id=\"hero\">\n  <h1>%s</h1>\n  <p>Replace this with your own content.</p>\n</section>\n", title),
				filepath.ToSlash(filepath.Join("styles", "tokens.css")):     ":root {\n  --page-bg: #f6f3ea;\n  --ink: #1e1b18;\n  --accent: #c35c2f;\n}\n",
				filepath.ToSlash(filepath.Join("styles", "default.css")):    "body {\n  margin: 0;\n  font-family: Georgia, serif;\n  background: var(--page-bg);\n  color: var(--ink);\n}\n\n#hero {\n  padding: 4rem 1.5rem;\n}\n",
			}
			for rel, content := range files {
				if err := writeInitFile(target, rel, content); err != nil {
					return err
				}
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Initialized minimal site in %s\n", target)
			fmt.Fprintf(cmd.OutOrStdout(), "Next: run 'htmlctl render -f %s -o %s'\n", target, filepath.ToSlash(filepath.Join(target, "dist")))
			return nil
		},
	}
	cmd.Flags().StringVar(&templateName, "template", "minimal", "Site template to generate")
	cmd.Flags().BoolVar(&force, "force", false, "Write into a non-empty target directory")
	return cmd
}

func newSiteExportCmd() *cobra.Command {
	var outputDir string
	var archivePath string
	var force bool
	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export remote desired state to a local site tree",
		Long:  "Export canonical source from the selected remote website/environment. YAML is regenerated canonically from desired state, so comments and original formatting are not preserved.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			rt, api, err := runtimeAndClientFromCommand(cmd)
			if err != nil {
				return err
			}
			website, err := requireContextWebsite(rt)
			if err != nil {
				return err
			}
			environment, err := requireContextEnvironment(rt)
			if err != nil {
				return err
			}
			if strings.TrimSpace(outputDir) == "" && strings.TrimSpace(archivePath) == "" {
				outputDir = fmt.Sprintf("%s-%s-site", website, environment)
			}
			if strings.TrimSpace(outputDir) != "" {
				if err := prepareExportDir(outputDir, force); err != nil {
					return err
				}
			}

			archiveStream, err := api.GetSourceArchive(cmd.Context(), website, environment)
			if err != nil {
				return err
			}
			defer archiveStream.Close()

			var archiveFile *os.File
			if strings.TrimSpace(archivePath) != "" {
				archiveFile, err = createArchiveFile(archivePath, force)
				if err != nil {
					return err
				}
			}

			input := io.Reader(archiveStream)
			if archiveFile != nil && strings.TrimSpace(outputDir) != "" {
				input = io.TeeReader(input, archiveFile)
			}

			if strings.TrimSpace(outputDir) != "" {
				if err := extractSourceArchive(outputDir, input); err != nil {
					if archiveFile != nil {
						_ = archiveFile.Close()
					}
					return err
				}
				if err := writeExportMarker(outputDir, website, environment); err != nil {
					if archiveFile != nil {
						_ = archiveFile.Close()
					}
					return err
				}
			} else if archiveFile != nil {
				if _, err := io.Copy(archiveFile, input); err != nil {
					_ = archiveFile.Close()
					return fmt.Errorf("write archive file %s: %w", archivePath, err)
				}
			}
			if archiveFile != nil {
				if err := archiveFile.Close(); err != nil {
					return fmt.Errorf("close archive file %s: %w", archivePath, err)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Wrote source archive to %s\n", archivePath)
			}
			if strings.TrimSpace(outputDir) != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "Exported site to %s\n", outputDir)
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Note: website.yaml and page YAML are regenerated canonically; comments and original formatting are not preserved.")
			return nil
		},
	}
	markRequiresTransport(cmd)
	cmd.Flags().StringVarP(&outputDir, "output", "o", "", "Directory to write the exported site tree")
	cmd.Flags().StringVar(&archivePath, "archive", "", "Optional path for the downloaded tar.gz archive")
	cmd.Flags().BoolVar(&force, "force", false, "Overwrite archive files or export into a non-empty directory")
	return cmd
}

func ensureInitTarget(target string, force bool) error {
	info, err := os.Stat(target)
	switch {
	case err == nil && !info.IsDir():
		return fmt.Errorf("target path %s exists and is not a directory", target)
	case err == nil:
		entries, readErr := os.ReadDir(target)
		if readErr != nil {
			return fmt.Errorf("read target directory %s: %w", target, readErr)
		}
		if len(entries) > 0 && !force {
			return fmt.Errorf("target directory %s is not empty; rerun with --force to write into it", target)
		}
	case os.IsNotExist(err):
		if mkErr := os.MkdirAll(target, 0o755); mkErr != nil {
			return fmt.Errorf("create target directory %s: %w", target, mkErr)
		}
	default:
		return fmt.Errorf("stat target directory %s: %w", target, err)
	}
	return nil
}

func writeInitFile(root, rel, content string) error {
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create directory for %s: %w", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write file %s: %w", path, err)
	}
	return nil
}

func deriveSiteName(target string) string {
	base := strings.TrimSpace(filepath.Base(target))
	if err := names.ValidateResourceName(base); err == nil {
		return base
	}
	return "sample"
}

func displayTitleForSite(name string) string {
	if strings.TrimSpace(name) == "" {
		return "Sample"
	}
	if len(name) == 1 {
		return strings.ToUpper(name)
	}
	return strings.ToUpper(name[:1]) + name[1:]
}

func createArchiveFile(path string, force bool) (*os.File, error) {
	if info, err := os.Stat(path); err == nil {
		if info.IsDir() {
			return nil, fmt.Errorf("archive path %s exists and is a directory", path)
		}
		if !force {
			return nil, fmt.Errorf("archive path %s already exists; rerun with --force to overwrite it", path)
		}
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("stat archive path %s: %w", path, err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create archive parent directory for %s: %w", path, err)
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open archive file %s: %w", path, err)
	}
	return file, nil
}

func prepareExportDir(root string, force bool) error {
	info, err := os.Stat(root)
	switch {
	case err == nil && !info.IsDir():
		return fmt.Errorf("output path %s exists and is not a directory", root)
	case err == nil:
		entries, readErr := os.ReadDir(root)
		if readErr != nil {
			return fmt.Errorf("read output directory %s: %w", root, readErr)
		}
		if len(entries) > 0 && !force {
			return fmt.Errorf("output directory %s is not empty; rerun with --force to write into it", root)
		}
		if len(entries) > 0 && force {
			if err := validateForceExportTarget(root); err != nil {
				return err
			}
			if _, err := os.Stat(filepath.Join(root, exportMarkerFile)); err != nil {
				if os.IsNotExist(err) {
					return fmt.Errorf("output directory %s is not a previous htmlctl export; choose a dedicated export directory or remove contents manually", root)
				}
				return fmt.Errorf("inspect export marker in %s: %w", root, err)
			}
			if err := clearDirectory(root); err != nil {
				return err
			}
		}
	case os.IsNotExist(err):
		if mkErr := os.MkdirAll(root, 0o755); mkErr != nil {
			return fmt.Errorf("create output directory %s: %w", root, mkErr)
		}
	default:
		return fmt.Errorf("stat output directory %s: %w", root, err)
	}
	return nil
}

func validateForceExportTarget(root string) error {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return fmt.Errorf("resolve output directory %s: %w", root, err)
	}
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("resolve current working directory: %w", err)
	}
	if absRoot == cwd {
		return fmt.Errorf("refusing to clear current working directory %s; choose a dedicated export directory", root)
	}
	if absRoot == filepath.Dir(absRoot) {
		return fmt.Errorf("refusing to clear filesystem root %s; choose a dedicated export directory", root)
	}
	if home, err := os.UserHomeDir(); err == nil && absRoot == home {
		return fmt.Errorf("refusing to clear home directory %s; choose a dedicated export directory", root)
	}
	return nil
}

func clearDirectory(root string) error {
	entries, err := os.ReadDir(root)
	if err != nil {
		return fmt.Errorf("read output directory %s: %w", root, err)
	}
	for _, entry := range entries {
		target := filepath.Join(root, entry.Name())
		if err := os.RemoveAll(target); err != nil {
			return fmt.Errorf("remove existing export entry %s: %w", target, err)
		}
	}
	return nil
}

func writeExportMarker(root, website, environment string) error {
	path := filepath.Join(root, exportMarkerFile)
	content := fmt.Sprintf("website=%s\nenvironment=%s\n", website, environment)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write export marker %s: %w", path, err)
	}
	return nil
}

func extractSourceArchive(root string, archive io.Reader) error {
	gzr, err := gzip.NewReader(archive)
	if err != nil {
		return fmt.Errorf("open source archive gzip stream: %w", err)
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("read source archive entry: %w", err)
		}
		rel, err := safeArchiveRelPath(hdr.Name)
		if err != nil {
			return err
		}
		target := filepath.Join(root, filepath.FromSlash(rel))
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return fmt.Errorf("create archive directory %s: %w", target, err)
			}
		case tar.TypeReg, tar.TypeRegA:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return fmt.Errorf("create archive file parent for %s: %w", target, err)
			}
			content, err := io.ReadAll(tr)
			if err != nil {
				return fmt.Errorf("read archive file %s: %w", rel, err)
			}
			if err := os.WriteFile(target, content, 0o644); err != nil {
				return fmt.Errorf("write archive file %s: %w", target, err)
			}
		default:
			return fmt.Errorf("unsupported source archive entry type for %s", rel)
		}
	}
}

func safeArchiveRelPath(name string) (string, error) {
	clean := filepath.ToSlash(filepath.Clean(strings.TrimSpace(name)))
	if clean == "." || clean == "" {
		return "", fmt.Errorf("source archive entry path is empty")
	}
	if len(clean) >= 3 && clean[1] == ':' && clean[2] == '/' && ((clean[0] >= 'a' && clean[0] <= 'z') || (clean[0] >= 'A' && clean[0] <= 'Z')) {
		return "", fmt.Errorf("invalid source archive entry path %q", name)
	}
	if strings.HasPrefix(clean, "/") || clean == ".." || strings.HasPrefix(clean, "../") || strings.Contains(clean, "/../") {
		return "", fmt.Errorf("invalid source archive entry path %q", name)
	}
	return clean, nil
}
