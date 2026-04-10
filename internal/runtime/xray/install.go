package xray

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	goruntime "runtime"
	"strings"

	"github.com/zhaodengfeng/dtsw/internal/ioutil"
)

type InstallOptions struct {
	DryRun bool
	Stdout io.Writer
	Stderr io.Writer
	GOOS   string
	GOARCH string
}

func AssetName(goos, goarch string) (string, error) {
	if goos != "linux" {
		return "", fmt.Errorf("unsupported target OS %q", goos)
	}
	switch goarch {
	case "amd64":
		return "Xray-linux-64.zip", nil
	case "arm64":
		return "Xray-linux-arm64-v8a.zip", nil
	default:
		return "", fmt.Errorf("unsupported target architecture %q", goarch)
	}
}

func DownloadURL(version, asset string) string {
	return fmt.Sprintf("https://github.com/XTLS/Xray-core/releases/download/%s/%s", version, asset)
}

func ChecksumURL(version, asset string) string {
	return DownloadURL(version, asset) + ".dgst"
}

func CurrentVersion(binary string) (string, error) {
	out, err := exec.Command(binary, "version").CombinedOutput()
	if err != nil {
		return "", err
	}
	fields := strings.Fields(string(out))
	if len(fields) < 2 {
		return "", fmt.Errorf("unexpected xray version output: %q", string(out))
	}
	version := fields[1]
	if !strings.HasPrefix(version, "v") {
		version = "v" + version
	}
	return version, nil
}

func Install(ctx context.Context, binaryPath, version string, opts InstallOptions) error {
	goos := opts.GOOS
	if goos == "" {
		goos = goruntime.GOOS
	}
	goarch := opts.GOARCH
	if goarch == "" {
		goarch = goruntime.GOARCH
	}
	asset, err := AssetName(goos, goarch)
	if err != nil {
		return err
	}
	url := DownloadURL(version, asset)
	checksumURL := ChecksumURL(version, asset)

	if opts.DryRun {
		if opts.Stdout != nil {
			fmt.Fprintf(opts.Stdout, "download %s\n", url)
			fmt.Fprintf(opts.Stdout, "verify %s\n", checksumURL)
			fmt.Fprintf(opts.Stdout, "install xray to %s\n", binaryPath)
		}
		return nil
	}

	tmpDir, err := os.MkdirTemp("", "dtsw-xray-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	archivePath := filepath.Join(tmpDir, asset)
	if err := ioutil.DownloadToFile(ctx, url, archivePath); err != nil {
		return err
	}
	checksumPath := filepath.Join(tmpDir, asset+".dgst")
	if err := ioutil.DownloadToFile(ctx, checksumURL, checksumPath); err != nil {
		return err
	}
	if err := verifyArchiveChecksum(archivePath, checksumPath); err != nil {
		return err
	}

	tmpBinary := filepath.Join(tmpDir, "xray")
	if err := extractBinary(archivePath, "xray", tmpBinary); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(binaryPath), 0o755); err != nil {
		return err
	}
	if err := os.Chmod(tmpBinary, 0o755); err != nil {
		return err
	}
	if err := ioutil.CopyFile(tmpBinary, binaryPath, 0o755); err != nil {
		return err
	}
	return nil
}

func verifyArchiveChecksum(archivePath, checksumPath string) error {
	expected, err := checksumFromFile(checksumPath)
	if err != nil {
		return err
	}
	actual, err := sha256File(archivePath)
	if err != nil {
		return err
	}
	if !strings.EqualFold(expected, actual) {
		return fmt.Errorf("xray archive checksum mismatch: expected %s got %s", expected, actual)
	}
	return nil
}

func checksumFromFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	re := regexp.MustCompile(`(?i)[a-f0-9]{64}`)
	match := re.FindString(string(data))
	if match == "" {
		return "", errors.New("failed to parse xray checksum file")
	}
	return strings.ToLower(match), nil
}

func sha256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func extractBinary(archivePath, name, outPath string) error {
	zr, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer zr.Close()
	for _, file := range zr.File {
		if file.FileInfo().IsDir() {
			continue
		}
		if filepath.Base(file.Name) != name {
			continue
		}
		rc, err := file.Open()
		if err != nil {
			return err
		}
		defer rc.Close()
		out, err := os.OpenFile(outPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
		if err != nil {
			return err
		}
		defer out.Close()
		if _, err := io.Copy(out, rc); err != nil {
			return err
		}
		return nil
	}
	return fmt.Errorf("xray binary not found in %s", archivePath)
}


