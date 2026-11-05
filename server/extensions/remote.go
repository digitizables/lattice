package extensions

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

// RemoteOptions describes a channel package fetched from the network or a local archive.
type RemoteOptions struct {
	// URL is an https:// (or http://) archive link, or a local .zip/.tar.gz path.
	URL string
	// GitHub is owner/repo or owner/repo@ref. Builds a codeload zipball URL.
	GitHub string
	// Ref overrides the ref for GitHub (default main). Ignored when GitHub contains @ref.
	Ref string
	// Subdir is the path inside the archive to the channel package (directory with manifest.yaml).
	Subdir string
}

// InstallRemoteChannel downloads or opens an archive, locates the channel package,
// and installs it via InstallLocalChannel.
func InstallRemoteChannel(extensionsDir string, opts RemoteOptions) (string, error) {
	srcURL, err := resolveRemoteURL(opts)
	if err != nil {
		return "", err
	}

	tmp, err := os.MkdirTemp("", "lattice-ext-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmp)

	archivePath := filepath.Join(tmp, "pkg"+archiveExt(srcURL))
	if local, ok := localArchivePath(srcURL); ok {
		if err := copyFile(local, archivePath, 0o644); err != nil {
			return "", fmt.Errorf("copy archive: %w", err)
		}
	} else {
		if err := downloadFile(srcURL, archivePath); err != nil {
			return "", err
		}
	}

	extractDir := filepath.Join(tmp, "extract")
	if err := os.MkdirAll(extractDir, 0o755); err != nil {
		return "", err
	}
	if err := extractArchive(archivePath, extractDir); err != nil {
		return "", err
	}

	pkgDir, err := findChannelPackage(extractDir, opts.Subdir)
	if err != nil {
		return "", err
	}
	return InstallLocalChannel(extensionsDir, pkgDir)
}

func resolveRemoteURL(opts RemoteOptions) (string, error) {
	if opts.URL != "" && opts.GitHub != "" {
		return "", fmt.Errorf("specify only one of -url or -github")
	}
	if opts.URL != "" {
		return strings.TrimSpace(opts.URL), nil
	}
	if opts.GitHub == "" {
		return "", fmt.Errorf("remote install requires -url or -github")
	}
	ownerRepo, ref := splitGitHubRef(opts.GitHub, opts.Ref)
	parts := strings.Split(ownerRepo, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", fmt.Errorf("github must be owner/repo or owner/repo@ref, got %q", opts.GitHub)
	}
	owner, repo := parts[0], parts[1]
	// Prefer tags when ref looks like a version; otherwise treat as branch.
	kind := "heads"
	if strings.HasPrefix(ref, "v") || looksLikeSemver(ref) {
		kind = "tags"
	}
	return fmt.Sprintf("https://codeload.github.com/%s/%s/zip/refs/%s/%s", owner, repo, kind, ref), nil
}

func splitGitHubRef(spec, flagRef string) (ownerRepo, ref string) {
	spec = strings.TrimSpace(spec)
	spec = strings.TrimPrefix(spec, "https://github.com/")
	spec = strings.TrimPrefix(spec, "http://github.com/")
	spec = strings.TrimSuffix(spec, ".git")
	ref = strings.TrimSpace(flagRef)
	if ref == "" {
		ref = "main"
	}
	if i := strings.LastIndex(spec, "@"); i > 0 {
		ownerRepo = spec[:i]
		ref = spec[i+1:]
		return ownerRepo, ref
	}
	return spec, ref
}

func looksLikeSemver(ref string) bool {
	if ref == "" {
		return false
	}
	dots := 0
	for _, c := range ref {
		if c == '.' {
			dots++
			continue
		}
		if c < '0' || c > '9' {
			return false
		}
	}
	return dots >= 1
}

func localArchivePath(u string) (string, bool) {
	u = strings.TrimSpace(u)
	if strings.HasPrefix(u, "https://") || strings.HasPrefix(u, "http://") {
		return "", false
	}
	if strings.HasPrefix(u, "file://") {
		p := strings.TrimPrefix(u, "file://")
		// file:///C:/path on Windows
		if len(p) >= 3 && p[0] == '/' && p[2] == ':' {
			p = p[1:]
		}
		return filepath.Clean(p), true
	}
	return filepath.Clean(u), true
}

func archiveExt(u string) string {
	clean := u
	if i := strings.Index(clean, "?"); i >= 0 {
		clean = clean[:i]
	}
	switch {
	case strings.HasSuffix(clean, ".tar.gz"), strings.HasSuffix(clean, ".tgz"):
		return ".tar.gz"
	default:
		return ".zip"
	}
}

func downloadFile(rawURL, dest string) error {
	client := &http.Client{Timeout: 120 * time.Second}
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "lattice-install-extension")
	res, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("download %s: %w", rawURL, err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: HTTP %s", rawURL, res.Status)
	}
	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, res.Body); err != nil {
		return err
	}
	return out.Close()
}

func extractArchive(archivePath, destDir string) error {
	lower := strings.ToLower(archivePath)
	switch {
	case strings.HasSuffix(lower, ".tar.gz"), strings.HasSuffix(lower, ".tgz"):
		return extractTarGz(archivePath, destDir)
	default:
		return extractZip(archivePath, destDir)
	}
}

func extractZip(archivePath, destDir string) error {
	r, err := zip.OpenReader(archivePath)
	if err != nil {
		return fmt.Errorf("open zip: %w", err)
	}
	defer r.Close()
	destClean := filepath.Clean(destDir)
	for _, f := range r.File {
		// Windows Compress-Archive may store backslashes; normalize to slash then local path.
		name := filepath.Clean(filepath.FromSlash(strings.ReplaceAll(f.Name, "\\", "/")))
		if name == "." || name == "" || strings.HasPrefix(name, "..") {
			continue
		}
		target := filepath.Join(destClean, name)
		if !strings.HasPrefix(target, destClean+string(os.PathSeparator)) && target != destClean {
			return fmt.Errorf("zip entry escapes destination: %s", f.Name)
		}
		if f.FileInfo().IsDir() || strings.HasSuffix(f.Name, "/") || strings.HasSuffix(f.Name, "\\") {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, f.Mode().Perm())
		if err != nil {
			rc.Close()
			return err
		}
		_, copyErr := io.Copy(out, rc)
		closeErr := out.Close()
		rc.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
	}
	return nil
}

func extractTarGz(archivePath, destDir string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("gzip: %w", err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		name := filepath.Clean(hdr.Name)
		if name == "." || strings.HasPrefix(name, "..") {
			continue
		}
		target := filepath.Join(destDir, name)
		if !strings.HasPrefix(target, filepath.Clean(destDir)+string(os.PathSeparator)) && target != filepath.Clean(destDir) {
			return fmt.Errorf("tar entry escapes destination: %s", hdr.Name)
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(hdr.Mode).Perm())
			if err != nil {
				return err
			}
			_, copyErr := io.Copy(out, tr)
			closeErr := out.Close()
			if copyErr != nil {
				return copyErr
			}
			if closeErr != nil {
				return closeErr
			}
		}
	}
}

func findChannelPackage(root, subdir string) (string, error) {
	if subdir != "" {
		dir := filepath.Join(root, filepath.FromSlash(path.Clean(subdir)))
		if _, err := loadManifest(filepath.Join(dir, "manifest.yaml")); err != nil {
			return "", fmt.Errorf("subdir %q: %w", subdir, err)
		}
		return dir, nil
	}

	var found []string
	_ = filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if info.Name() != "manifest.yaml" {
			return nil
		}
		m, err := loadManifest(p)
		if err != nil || m.Kind != "channel" {
			return nil
		}
		found = append(found, filepath.Dir(p))
		return nil
	})
	switch len(found) {
	case 0:
		return "", fmt.Errorf("no channel manifest.yaml found in archive (pass -subdir)")
	case 1:
		return found[0], nil
	default:
		return "", fmt.Errorf("multiple channel packages in archive (%d); pass -subdir", len(found))
	}
}
