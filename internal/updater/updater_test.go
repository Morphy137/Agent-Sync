package updater_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"agentsync/internal/apperr"
	"agentsync/internal/updater"
)

const installerBody = "fake installer bytes"

type fakeGitHub struct {
	server    *httptest.Server
	checksums string
}

func newFakeGitHub(t *testing.T, releases func(base string) any) *fakeGitHub {
	t.Helper()
	f := &fakeGitHub{}
	sum := sha256.Sum256([]byte(installerBody))
	f.checksums = hex.EncodeToString(sum[:]) + " *" + updater.InstallerAssetName + "\n"
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/owner/repo/releases", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(releases(f.server.URL))
	})
	mux.HandleFunc("/dl/"+updater.InstallerAssetName, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, installerBody)
	})
	mux.HandleFunc("/dl/"+updater.ChecksumsAssetName, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, f.checksums)
	})
	f.server = httptest.NewServer(mux)
	t.Cleanup(f.server.Close)
	return f
}

func (f *fakeGitHub) client() *updater.Client {
	u, _ := url.Parse(f.server.URL)
	return &updater.Client{
		APIBase:       f.server.URL,
		Repo:          "owner/repo",
		HTTP:          f.server.Client(),
		DownloadHosts: []string{u.Hostname()},
		AllowHTTP:     true,
	}
}

func release(base, tag string, draft, prerelease bool) map[string]any {
	return map[string]any{
		"tag_name": tag, "name": "AgentSync " + tag, "body": "notes", "draft": draft,
		"prerelease": prerelease, "html_url": base + "/release/" + tag,
		"assets": []map[string]string{
			{"name": updater.InstallerAssetName, "browser_download_url": base + "/dl/" + updater.InstallerAssetName},
			{"name": updater.ChecksumsAssetName, "browser_download_url": base + "/dl/" + updater.ChecksumsAssetName},
		},
	}
}

func TestLatestPicksHighestPublishedVersion(t *testing.T) {
	gh := newFakeGitHub(t, func(base string) any {
		return []any{
			release(base, "v0.0.2", false, true),
			release(base, "v0.1.0", true, false), // draft: ignored
			release(base, "v0.0.10", false, false),
			release(base, "nightly", false, false), // not a version
		}
	})
	latest, err := gh.client().Latest(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if latest.Version != "0.0.10" || latest.InstallerURL == "" || latest.ChecksumsURL == "" {
		t.Fatalf("unexpected latest: %+v", latest)
	}
	if !updater.IsNewer(latest, "0.0.2") || updater.IsNewer(latest, "v0.0.10") {
		t.Fatal("IsNewer compares numerically")
	}
	if updater.IsNewer(latest, "dev") {
		t.Fatal("development builds never auto-update")
	}
}

func TestDownloadVerifiesChecksum(t *testing.T) {
	gh := newFakeGitHub(t, func(base string) any { return []any{release(base, "v0.0.3", false, false)} })
	c := gh.client()
	latest, err := c.Latest(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	path, err := c.Download(context.Background(), latest, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	if string(data) != installerBody {
		t.Fatalf("downloaded %q", data)
	}

	gh.checksums = strings.Repeat("0", 64) + " *" + updater.InstallerAssetName + "\n"
	dir := t.TempDir()
	_, err = c.Download(context.Background(), latest, dir)
	if !apperr.HasCode(err, apperr.UpdateVerifyFailed) {
		t.Fatalf("expected verify failure, got %v", err)
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Fatal("a file that fails verification must be deleted")
	}
}

func TestDownloadRejectsUntrustedHosts(t *testing.T) {
	gh := newFakeGitHub(t, func(base string) any { return []any{release(base, "v0.0.3", false, false)} })
	c := gh.client()
	latest, _ := c.Latest(context.Background())
	latest.InstallerURL = "http://evil.example/" + updater.InstallerAssetName
	if _, err := c.Download(context.Background(), latest, t.TempDir()); !apperr.HasCode(err, apperr.UpdateDownloadFailed) {
		t.Fatalf("expected untrusted host to be refused, got %v", err)
	}

	strict := updater.NewClient()
	if _, err := strict.Download(context.Background(), &updater.Release{
		InstallerURL: "http://github.com/x", ChecksumsURL: "http://github.com/y",
	}, t.TempDir()); err == nil {
		t.Fatal("plain http must be refused by the default client")
	}
}

func TestCompareVersions(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"0.0.2", "0.0.1", 1},
		{"v0.0.10", "0.0.9", 1},
		{"1.0.0-beta", "1.0.0", -1},
		{"1.2.3", "v1.2.3", 0},
		{"bad", "0.0.1", -1},
	}
	for _, c := range cases {
		if got := updater.CompareVersions(c.a, c.b); got != c.want {
			t.Errorf("CompareVersions(%q, %q) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

func TestParseChecksums(t *testing.T) {
	sums := updater.ParseChecksums("ABC *agentsync.exe\ndef  agentsync-setup.exe\nnoise\n")
	if sums["agentsync.exe"] != "abc" || sums["agentsync-setup.exe"] != "def" || len(sums) != 2 {
		t.Fatalf("unexpected: %v", sums)
	}
}
