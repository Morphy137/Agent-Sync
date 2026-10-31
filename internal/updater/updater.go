// Package updater finds newer AgentSync releases on GitHub, downloads the installer and checks
// its SHA-256 against the SHA256SUMS.txt published with the same release.
package updater

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"agentsync/internal/apperr"
)

const (
	// DefaultAPIBase is the GitHub REST API root.
	DefaultAPIBase = "https://api.github.com"
	// DefaultRepo is the public repository that publishes AgentSync releases.
	DefaultRepo = "Morphy137/Agent-Sync"
	// InstallerAssetName is the per-user NSIS installer attached to every release.
	InstallerAssetName = "agentsync-setup.exe"
	// ChecksumsAssetName lists "<sha256> *<file>" for every release asset.
	ChecksumsAssetName = "SHA256SUMS.txt"

	releasesPerPage     = 20
	requestTimeout      = 30 * time.Second
	downloadTimeout     = 10 * time.Minute
	maxInstallerBytes   = 300 << 20
	maxMetadataBytes    = 2 << 20
	versionPartsCount   = 3
	githubAcceptHeader  = "application/vnd.github+json"
	downloadFilePattern = "agentsync-update-*.exe"
)

// defaultDownloadHosts are the hosts GitHub serves release assets from (after redirects).
var defaultDownloadHosts = []string{
	"github.com",
	"objects.githubusercontent.com",
	"release-assets.githubusercontent.com",
}

// Release is a published version that can be installed.
type Release struct {
	Version      string `json:"version"`
	Tag          string `json:"tag"`
	Name         string `json:"name"`
	Notes        string `json:"notes"`
	PageURL      string `json:"pageUrl"`
	PublishedAt  string `json:"publishedAt"`
	Prerelease   bool   `json:"prerelease"`
	InstallerURL string `json:"installerUrl"`
	ChecksumsURL string `json:"checksumsUrl"`
}

// Client talks to the GitHub releases API.
type Client struct {
	APIBase string
	Repo    string
	HTTP    *http.Client
	// DownloadHosts limits where assets may be downloaded from (redirects included).
	DownloadHosts []string
	// AllowHTTP permits plain http URLs; only tests use it.
	AllowHTTP bool
}

// NewClient returns a client for the public AgentSync repository.
func NewClient() *Client {
	return &Client{
		APIBase:       DefaultAPIBase,
		Repo:          DefaultRepo,
		HTTP:          &http.Client{Timeout: requestTimeout},
		DownloadHosts: defaultDownloadHosts,
	}
}

type githubAsset struct {
	Name string `json:"name"`
	URL  string `json:"browser_download_url"`
}

type githubRelease struct {
	TagName     string        `json:"tag_name"`
	Name        string        `json:"name"`
	Body        string        `json:"body"`
	HTMLURL     string        `json:"html_url"`
	Draft       bool          `json:"draft"`
	Prerelease  bool          `json:"prerelease"`
	PublishedAt string        `json:"published_at"`
	Assets      []githubAsset `json:"assets"`
}

// Latest returns the newest non-draft release (pre-releases included, since AgentSync is 0.x).
func (c *Client) Latest(ctx context.Context) (*Release, error) {
	endpoint := fmt.Sprintf("%s/repos/%s/releases?per_page=%d", strings.TrimRight(c.APIBase, "/"), c.Repo, releasesPerPage)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, apperr.Wrap(apperr.UpdateCheckFailed, err)
	}
	req.Header.Set("Accept", githubAcceptHeader)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, apperr.Wrap(apperr.UpdateCheckFailed, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, apperr.New(apperr.UpdateCheckFailed).WithParam("status", strconv.Itoa(resp.StatusCode))
	}
	var releases []githubRelease
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxMetadataBytes)).Decode(&releases); err != nil {
		return nil, apperr.Wrap(apperr.UpdateCheckFailed, err)
	}

	var best *Release
	for _, r := range releases {
		version := NormalizeVersion(r.TagName)
		if r.Draft || !IsValidVersion(version) {
			continue
		}
		if best != nil && CompareVersions(version, best.Version) <= 0 {
			continue
		}
		candidate := &Release{
			Version: version, Tag: r.TagName, Name: r.Name, Notes: r.Body, PageURL: r.HTMLURL,
			PublishedAt: r.PublishedAt, Prerelease: r.Prerelease,
		}
		for _, a := range r.Assets {
			switch a.Name {
			case InstallerAssetName:
				candidate.InstallerURL = a.URL
			case ChecksumsAssetName:
				candidate.ChecksumsURL = a.URL
			}
		}
		best = candidate
	}
	return best, nil
}

// IsNewer reports whether release is newer than the running version.
func IsNewer(release *Release, current string) bool {
	if release == nil || !IsValidVersion(NormalizeVersion(current)) {
		return false
	}
	return CompareVersions(release.Version, NormalizeVersion(current)) > 0
}

// Download fetches the installer into dir and verifies it against the release checksums.
// It returns the path of the verified installer.
func (c *Client) Download(ctx context.Context, release *Release, dir string) (string, error) {
	if release == nil || release.InstallerURL == "" || release.ChecksumsURL == "" {
		return "", apperr.New(apperr.UpdateUnavailable)
	}
	ctx, cancel := context.WithTimeout(ctx, downloadTimeout)
	defer cancel()

	sums, err := c.fetchText(ctx, release.ChecksumsURL)
	if err != nil {
		return "", err
	}
	expected, ok := ParseChecksums(sums)[InstallerAssetName]
	if !ok {
		return "", apperr.New(apperr.UpdateVerifyFailed).WithParam("reason", "checksum_missing")
	}

	resp, err := c.get(ctx, release.InstallerURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	out, err := os.CreateTemp(dir, downloadFilePattern)
	if err != nil {
		return "", apperr.Wrap(apperr.WriteFailed, err)
	}
	path := out.Name()
	hasher := sha256.New()
	written, copyErr := io.Copy(io.MultiWriter(out, hasher), io.LimitReader(resp.Body, maxInstallerBytes+1))
	closeErr := out.Close()
	if copyErr != nil || closeErr != nil {
		_ = os.Remove(path)
		return "", apperr.Wrap(apperr.UpdateDownloadFailed, firstErr(copyErr, closeErr))
	}
	if written > maxInstallerBytes {
		_ = os.Remove(path)
		return "", apperr.New(apperr.UpdateDownloadFailed).WithParam("reason", "too_large")
	}
	if got := hex.EncodeToString(hasher.Sum(nil)); !strings.EqualFold(got, expected) {
		_ = os.Remove(path)
		return "", apperr.New(apperr.UpdateVerifyFailed).WithParam("reason", "checksum_mismatch")
	}
	return path, nil
}

// ParseChecksums reads "sha256sum" output ("<hex> *name" or "<hex>  name") into name -> hex.
func ParseChecksums(text string) map[string]string {
	sums := make(map[string]string)
	scanner := bufio.NewScanner(strings.NewReader(text))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) != 2 {
			continue
		}
		name := strings.TrimPrefix(fields[1], "*")
		sums[filepath.Base(name)] = strings.ToLower(fields[0])
	}
	return sums
}

func (c *Client) fetchText(ctx context.Context, rawURL string) (string, error) {
	resp, err := c.get(ctx, rawURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxMetadataBytes))
	if err != nil {
		return "", apperr.Wrap(apperr.UpdateDownloadFailed, err)
	}
	return string(data), nil
}

// get downloads from an allowed host, re-checking every redirect hop.
func (c *Client) get(ctx context.Context, rawURL string) (*http.Response, error) {
	if err := c.checkURL(rawURL); err != nil {
		return nil, err
	}
	client := *c.HTTP
	client.Timeout = 0 // the context bounds downloads
	client.CheckRedirect = func(req *http.Request, _ []*http.Request) error {
		return c.checkURL(req.URL.String())
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, apperr.Wrap(apperr.UpdateDownloadFailed, err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, apperr.Wrap(apperr.UpdateDownloadFailed, err)
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, apperr.New(apperr.UpdateDownloadFailed).WithParam("status", strconv.Itoa(resp.StatusCode))
	}
	return resp, nil
}

func (c *Client) checkURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return apperr.Wrap(apperr.UpdateDownloadFailed, err)
	}
	if u.Scheme != "https" && !(c.AllowHTTP && u.Scheme == "http") {
		return apperr.New(apperr.UpdateDownloadFailed).WithParam("reason", "insecure_url")
	}
	host := strings.ToLower(u.Hostname())
	for _, allowed := range c.DownloadHosts {
		if host == strings.ToLower(allowed) {
			return nil
		}
	}
	return apperr.New(apperr.UpdateDownloadFailed).WithParam("reason", "untrusted_host").WithParam("host", host)
}

// NormalizeVersion strips a leading "v" and surrounding spaces.
func NormalizeVersion(v string) string {
	return strings.TrimPrefix(strings.TrimSpace(v), "v")
}

// IsValidVersion accepts MAJOR.MINOR.PATCH with an optional "-suffix".
func IsValidVersion(v string) bool {
	_, ok := versionParts(v)
	return ok
}

// CompareVersions compares MAJOR.MINOR.PATCH; a version with a "-suffix" sorts before the
// same version without one. Invalid versions sort first.
func CompareVersions(a, b string) int {
	pa, okA := versionParts(a)
	pb, okB := versionParts(b)
	switch {
	case !okA && !okB:
		return 0
	case !okA:
		return -1
	case !okB:
		return 1
	}
	for i := 0; i < versionPartsCount; i++ {
		if pa[i] != pb[i] {
			if pa[i] < pb[i] {
				return -1
			}
			return 1
		}
	}
	preA, preB := hasPrerelease(a), hasPrerelease(b)
	switch {
	case preA && !preB:
		return -1
	case !preA && preB:
		return 1
	}
	return 0
}

func versionParts(v string) ([versionPartsCount]int, bool) {
	var parts [versionPartsCount]int
	core := strings.SplitN(NormalizeVersion(v), "-", 2)[0]
	fields := strings.Split(core, ".")
	if len(fields) != versionPartsCount {
		return parts, false
	}
	for i, f := range fields {
		n, err := strconv.Atoi(f)
		if err != nil || n < 0 {
			return parts, false
		}
		parts[i] = n
	}
	return parts, true
}

func hasPrerelease(v string) bool {
	return strings.Contains(NormalizeVersion(v), "-")
}

func firstErr(errs ...error) error {
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}
