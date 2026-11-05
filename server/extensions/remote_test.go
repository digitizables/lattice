package extensions

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
)

func TestInstallRemoteChannelFromZip(t *testing.T) {
	tmp := t.TempDir()
	pkg := filepath.Join(tmp, "pkg")
	if err := os.MkdirAll(pkg, 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := "name: zip-fixture\nkind: channel\nversion: 9.9.9\nenabled: false\n"
	if err := os.WriteFile(filepath.Join(pkg, "manifest.yaml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	zipPath := filepath.Join(tmp, "fixture.zip")
	if err := writeZip(zipPath, map[string]string{
		"fixture/manifest.yaml": manifest,
		"fixture/channel.go":    "package fixture\n",
	}); err != nil {
		t.Fatal(err)
	}

	extRoot := filepath.Join(tmp, "extensions")
	dst, err := InstallRemoteChannel(extRoot, RemoteOptions{URL: zipPath, Subdir: "fixture"})
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(extRoot, "channels", "zip-fixture")
	if dst != want {
		t.Fatalf("dst=%s want=%s", dst, want)
	}
	raw, err := os.ReadFile(filepath.Join(want, "manifest.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != manifest {
		t.Fatalf("manifest mismatch: %q", raw)
	}
}

func TestSplitGitHubRef(t *testing.T) {
	ownerRepo, ref := splitGitHubRef("acme/channel@v1.2.3", "")
	if ownerRepo != "acme/channel" || ref != "v1.2.3" {
		t.Fatalf("got %s %s", ownerRepo, ref)
	}
	ownerRepo, ref = splitGitHubRef("acme/channel", "develop")
	if ownerRepo != "acme/channel" || ref != "develop" {
		t.Fatalf("got %s %s", ownerRepo, ref)
	}
}

func writeZip(path string, files map[string]string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	zw := zip.NewWriter(f)
	for name, body := range files {
		w, err := zw.Create(name)
		if err != nil {
			return err
		}
		if _, err := w.Write([]byte(body)); err != nil {
			return err
		}
	}
	return zw.Close()
}
