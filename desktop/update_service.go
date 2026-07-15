package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const desktopUpdateCredentialRef = "secret://updates/github/genesis"

var desktopVersion = "0.1.40"

var ErrUpdateCredentialRequired = errors.New("update credential is required")

type DesktopUpdateProjection struct {
	CurrentVersion string `json:"current_version"`
	LatestVersion  string `json:"latest_version,omitempty"`
	ReleaseURL     string `json:"release_url,omitempty"`
	ReleaseNotes   string `json:"release_notes,omitempty"`
	InstallerURL   string `json:"installer_url,omitempty"`
	ChecksumURL    string `json:"checksum_url,omitempty"`
	Available      bool   `json:"available"`
	Reason         string `json:"reason,omitempty"`
}

type desktopUpdateService struct {
	currentVersion   string
	latestReleaseURL string
	client           *http.Client
	tokenResolver    func() (string, error)
	downloadDir      string
}

func (s desktopUpdateService) DownloadAndVerify(ctx context.Context, update DesktopUpdateProjection) (string, error) {
	token := s.updateToken()
	checksum, err := s.download(ctx, update.ChecksumURL, token)
	if err != nil {
		return "", err
	}
	expected := strings.Fields(string(checksum))
	if len(expected) == 0 || len(expected[0]) != 64 {
		return "", errors.New("update checksum is invalid")
	}
	installer, err := s.download(ctx, update.InstallerURL, token)
	if err != nil {
		return "", err
	}
	actual := fmt.Sprintf("%x", sha256.Sum256(installer))
	if !strings.EqualFold(actual, expected[0]) {
		return "", errors.New("update checksum mismatch")
	}
	dir := s.downloadDir
	if dir == "" {
		dir = filepath.Join(os.TempDir(), "genesis-desktop", "updates")
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	path := filepath.Join(dir, "genesis-desktop-update.exe")
	if err := os.WriteFile(path, installer, 0o700); err != nil {
		return "", err
	}
	return path, nil
}

func (s desktopUpdateService) updateToken() string {
	if s.tokenResolver == nil {
		return ""
	}
	token, err := s.tokenResolver()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(token)
}

func (s desktopUpdateService) download(ctx context.Context, location, token string) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimSpace(location), nil)
	if err != nil {
		return nil, err
	}
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	client := s.client
	if client == nil {
		client = http.DefaultClient
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, updateHTTPError(response.StatusCode, token, "download")
	}
	return io.ReadAll(response.Body)
}

func (s desktopUpdateService) Check(ctx context.Context) (DesktopUpdateProjection, error) {
	result := DesktopUpdateProjection{CurrentVersion: strings.TrimSpace(s.currentVersion)}
	if result.CurrentVersion == "" {
		result.CurrentVersion = "dev"
	}
	token := s.updateToken()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimSpace(s.latestReleaseURL), nil)
	if err != nil {
		return result, err
	}
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	client := s.client
	if client == nil {
		client = http.DefaultClient
	}
	response, err := client.Do(request)
	if err != nil {
		return result, err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return result, updateHTTPError(response.StatusCode, token, "release request")
	}
	var release struct {
		TagName string `json:"tag_name"`
		HTMLURL string `json:"html_url"`
		Body    string `json:"body"`
		Assets  []struct {
			Name string `json:"name"`
			URL  string `json:"browser_download_url"`
		} `json:"assets"`
	}
	if err := json.NewDecoder(response.Body).Decode(&release); err != nil {
		return result, err
	}
	result.LatestVersion = strings.TrimPrefix(strings.TrimSpace(release.TagName), "v")
	result.ReleaseURL = strings.TrimSpace(release.HTMLURL)
	result.ReleaseNotes = strings.TrimSpace(release.Body)
	for _, asset := range release.Assets {
		switch strings.TrimSpace(asset.Name) {
		case "genesis-desktop-amd64-installer.exe":
			result.InstallerURL = strings.TrimSpace(asset.URL)
		case "genesis-desktop-amd64-installer.exe.sha256":
			result.ChecksumURL = strings.TrimSpace(asset.URL)
		}
	}
	if result.LatestVersion == "" || result.InstallerURL == "" || result.ChecksumURL == "" {
		return result, errors.New("latest release is incomplete")
	}
	result.Available = compareDesktopVersions(result.CurrentVersion, result.LatestVersion) < 0
	return result, nil
}

func updateHTTPError(status int, token string, operation string) error {
	if operation == "release request" && strings.TrimSpace(token) == "" && (status == http.StatusUnauthorized || status == http.StatusNotFound) {
		return ErrUpdateCredentialRequired
	}
	return fmt.Errorf("update %s returned HTTP %d", operation, status)
}

func compareDesktopVersions(current, latest string) int {
	parse := func(value string) []int {
		parts := strings.Split(strings.TrimPrefix(strings.TrimSpace(value), "v"), ".")
		result := make([]int, 3)
		for index, part := range parts {
			if index == len(result) {
				break
			}
			result[index], _ = strconv.Atoi(part)
		}
		return result
	}
	left, right := parse(current), parse(latest)
	for index := range left {
		if left[index] < right[index] {
			return -1
		}
		if left[index] > right[index] {
			return 1
		}
	}
	return 0
}
