package updater

import (
	"errors"
	"fmt"
	"io"
	"net/url"
	"runtime"
	"strings"

	"github.com/goccy/go-json"
)

// Releases are fetched from this fork's GitHub repository. The upstream
// seanime.app release channels are not used: they publish upstream builds,
// which would replace a fork build with an upstream one.
var (
	githubReleaseUrl    = "https://api.github.com/repos/carneirofc/seanime/releases/latest"
	ErrInsecureUpdateURL = errors.New("update URL must use https")
)

func validateUpdateURL(rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid update URL %q: %w", rawURL, err)
	}

	if parsed.Scheme != "https" || parsed.Host == "" {
		return fmt.Errorf("%w: %s", ErrInsecureUpdateURL, rawURL)
	}

	return nil
}

func validateReleaseDownloadURLs(release *Release) error {
	for _, asset := range release.Assets {
		if err := validateUpdateURL(asset.BrowserDownloadUrl); err != nil {
			return err
		}
	}

	return nil
}

type (
	GitHubResponse struct {
		Url             string `json:"url"`
		AssetsUrl       string `json:"assets_url"`
		UploadUrl       string `json:"upload_url"`
		HtmlUrl         string `json:"html_url"`
		ID              int64  `json:"id"`
		NodeID          string `json:"node_id"`
		TagName         string `json:"tag_name"`
		TargetCommitish string `json:"target_commitish"`
		Name            string `json:"name"`
		Draft           bool   `json:"draft"`
		Prerelease      bool   `json:"prerelease"`
		CreatedAt       string `json:"created_at"`
		PublishedAt     string `json:"published_at"`
		Assets          []struct {
			Url                string `json:"url"`
			ID                 int64  `json:"id"`
			NodeID             string `json:"node_id"`
			Name               string `json:"name"`
			Label              string `json:"label"`
			ContentType        string `json:"content_type"`
			State              string `json:"state"`
			Size               int64  `json:"size"`
			DownloadCount      int64  `json:"download_count"`
			CreatedAt          string `json:"created_at"`
			UpdatedAt          string `json:"updated_at"`
			BrowserDownloadURL string `json:"browser_download_url"`
		} `json:"assets"`
		TarballURL string `json:"tarball_url"`
		ZipballURL string `json:"zipball_url"`
		Body       string `json:"body"`
	}

	Release struct {
		Url         string         `json:"url"`
		HtmlUrl     string         `json:"html_url"`
		NodeId      string         `json:"node_id"`
		TagName     string         `json:"tag_name"`
		Name        string         `json:"name"`
		Body        string         `json:"body"`
		PublishedAt string         `json:"published_at"`
		Released    bool           `json:"released"`
		Version     string         `json:"version"`
		Assets      []ReleaseAsset `json:"assets"`
	}
	ReleaseAsset struct {
		Url                string `json:"url"`
		Id                 int64  `json:"id"`
		NodeId             string `json:"node_id"`
		Name               string `json:"name"`
		ContentType        string `json:"content_type"`
		Uploaded           bool   `json:"uploaded"`
		Size               int64  `json:"size"`
		BrowserDownloadUrl string `json:"browser_download_url"`
	}
)

func (u *Updater) GetReleaseName(version string) string {

	arch := runtime.GOARCH
	switch runtime.GOARCH {
	case "amd64":
		arch = "x86_64"
	case "arm64":
		arch = "arm64"
	case "386":
		return "i386"
	}
	oos := runtime.GOOS
	switch runtime.GOOS {
	case "linux":
		oos = "Linux"
	case "windows":
		oos = "Windows"
	case "darwin":
		oos = "MacOS"
	}

	ext := "tar.gz"
	if oos == "Windows" {
		ext = "zip"
	}

	return fmt.Sprintf("seanime-%s_%s_%s.%s", version, oos, arch, ext)
}

// fetchLatestRelease returns the latest release of this fork. The channel is
// accepted for compatibility with stored settings ("seanime", "seanime_nightly"
// were upstream-hosted channels) but every channel resolves to GitHub.
func (u *Updater) fetchLatestRelease(_ string) (*Release, error) {
	return u.fetchLatestReleaseFromGitHub()
}

func (u *Updater) fetchLatestReleaseFromGitHub() (*Release, error) {
	if err := validateUpdateURL(githubReleaseUrl); err != nil {
		return nil, err
	}

	response, err := u.client.Get(githubReleaseUrl)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode > 299 {
		return nil, fmt.Errorf("github releases: http error code: %d", response.StatusCode)
	}

	byteArr, readErr := io.ReadAll(response.Body)
	if readErr != nil {
		return nil, fmt.Errorf("error reading response: %w\n", readErr)
	}

	var res GitHubResponse
	err = json.Unmarshal(byteArr, &res)
	if err != nil {
		return nil, err
	}

	release := &Release{
		Url:         res.Url,
		HtmlUrl:     res.HtmlUrl,
		NodeId:      res.NodeID,
		TagName:     res.TagName,
		Name:        res.Name,
		Body:        res.Body,
		PublishedAt: res.PublishedAt,
		Released:    !res.Prerelease && !res.Draft,
		Version:     strings.TrimPrefix(res.TagName, "v"),
		Assets:      make([]ReleaseAsset, len(res.Assets)),
	}

	for i, asset := range res.Assets {
		release.Assets[i] = ReleaseAsset{
			Url:                asset.Url,
			Id:                 asset.ID,
			NodeId:             asset.NodeID,
			Name:               asset.Name,
			ContentType:        asset.ContentType,
			Uploaded:           asset.State == "uploaded",
			Size:               asset.Size,
			BrowserDownloadUrl: asset.BrowserDownloadURL,
		}
	}

	if err := validateReleaseDownloadURLs(release); err != nil {
		return nil, err
	}

	return release, nil
}
