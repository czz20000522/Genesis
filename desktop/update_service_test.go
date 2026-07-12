package main

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestCompareDesktopVersions(t *testing.T) {
	if got := compareDesktopVersions("0.1.7", "v0.1.8"); got >= 0 {
		t.Fatalf("compareDesktopVersions = %d, want newer release", got)
	}
	if got := compareDesktopVersions("0.1.7", "v0.1.7"); got != 0 {
		t.Fatalf("compareDesktopVersions = %d, want equal versions", got)
	}
}

func TestDesktopUpdateServiceDownloadsOnlyVerifiedInstaller(t *testing.T) {
	payload := []byte("verified installer")
	digest := fmt.Sprintf("%x", sha256.Sum256(payload))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer protected-token" {
			t.Fatal("download did not use update token")
		}
		switch r.URL.Path {
		case "/installer.exe":
			_, _ = w.Write(payload)
		case "/installer.sha256":
			_, _ = w.Write([]byte(digest + " *genesis-desktop-amd64-installer.exe\n"))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	service := desktopUpdateService{client: server.Client(), tokenResolver: func() (string, error) { return "protected-token", nil }, downloadDir: t.TempDir()}
	path, err := service.DownloadAndVerify(context.Background(), DesktopUpdateProjection{InstallerURL: server.URL + "/installer.exe", ChecksumURL: server.URL + "/installer.sha256"})
	if err != nil {
		t.Fatalf("DownloadAndVerify returned error: %v", err)
	}
	if payload, err := os.ReadFile(path); err != nil || string(payload) != "verified installer" {
		t.Fatalf("installer payload = %q, err = %v", payload, err)
	}
}

func TestDesktopUpdateServiceChecksNewerRelease(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer protected-token" {
			t.Fatalf("authorization = %q", got)
		}
		_, _ = w.Write([]byte(`{"tag_name":"v0.1.8","html_url":"https://example.invalid/release","body":"notes","assets":[{"name":"genesis-desktop-amd64-installer.exe","browser_download_url":"https://example.invalid/installer.exe","size":123},{"name":"genesis-desktop-amd64-installer.exe.sha256","browser_download_url":"https://example.invalid/installer.sha256","size":64}]}`))
	}))
	defer server.Close()

	service := desktopUpdateService{currentVersion: "0.1.7", latestReleaseURL: server.URL, tokenResolver: func() (string, error) { return "protected-token", nil }}
	update, err := service.Check(context.Background())
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}
	if !update.Available || update.LatestVersion != "0.1.8" || update.InstallerURL == "" || update.ChecksumURL == "" {
		t.Fatalf("update = %+v", update)
	}
}

func TestDesktopUpdateServiceChecksAndDownloadsPublicReleaseWithoutToken(t *testing.T) {
	payload := []byte("public installer")
	digest := fmt.Sprintf("%x", sha256.Sum256(payload))
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "" {
			t.Fatalf("public update sent authorization = %q", got)
		}
		switch r.URL.Path {
		case "/latest":
			_, _ = w.Write([]byte(`{"tag_name":"v0.1.9","assets":[{"name":"genesis-desktop-amd64-installer.exe","browser_download_url":"` + server.URL + `/installer.exe"},{"name":"genesis-desktop-amd64-installer.exe.sha256","browser_download_url":"` + server.URL + `/installer.sha256"}]}`))
		case "/installer.exe":
			_, _ = w.Write(payload)
		case "/installer.sha256":
			_, _ = w.Write([]byte(digest))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	service := desktopUpdateService{currentVersion: "0.1.8", latestReleaseURL: server.URL + "/latest", client: server.Client(), downloadDir: t.TempDir()}
	update, err := service.Check(context.Background())
	if err != nil || !update.Available {
		t.Fatalf("public update = %+v, err = %v", update, err)
	}
	if _, err := service.DownloadAndVerify(context.Background(), update); err != nil {
		t.Fatalf("download public update: %v", err)
	}
}
