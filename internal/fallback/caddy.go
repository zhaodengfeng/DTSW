package fallback

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	goruntime "runtime"
	"strings"

	"github.com/zhaodengfeng/dtsw/internal/config"
	"github.com/zhaodengfeng/dtsw/internal/ioutil"
)

type InstallOptions struct {
	DryRun bool
	Stdout io.Writer
	Stderr io.Writer
	GOOS   string
	GOARCH string
}

func CaddyAssetName(version, goos, goarch string) (string, error) {
	if goos != "linux" {
		return "", fmt.Errorf("unsupported target OS %q", goos)
	}
	version = strings.TrimPrefix(version, "v")
	switch goarch {
	case "amd64", "arm64":
		return fmt.Sprintf("caddy_%s_linux_%s.tar.gz", version, goarch), nil
	default:
		return "", fmt.Errorf("unsupported target architecture %q", goarch)
	}
}

func CaddyDownloadURL(version, asset string) string {
	return fmt.Sprintf("https://github.com/caddyserver/caddy/releases/download/%s/%s", version, asset)
}

func CaddyChecksumsURL(version string) string {
	trimmed := strings.TrimPrefix(version, "v")
	return fmt.Sprintf("https://github.com/caddyserver/caddy/releases/download/%s/caddy_%s_checksums.txt", version, trimmed)
}

func InstallCaddy(ctx context.Context, binaryPath string, opts InstallOptions) error {
	goos := opts.GOOS
	if goos == "" {
		goos = goruntime.GOOS
	}
	goarch := opts.GOARCH
	if goarch == "" {
		goarch = goruntime.GOARCH
	}
	asset, err := CaddyAssetName(config.DefaultCaddyVersion, goos, goarch)
	if err != nil {
		return err
	}
	url := CaddyDownloadURL(config.DefaultCaddyVersion, asset)
	checksumsURL := CaddyChecksumsURL(config.DefaultCaddyVersion)

	if opts.DryRun {
		if opts.Stdout != nil {
			fmt.Fprintf(opts.Stdout, "download %s\n", url)
			fmt.Fprintf(opts.Stdout, "verify %s\n", checksumsURL)
			fmt.Fprintf(opts.Stdout, "install caddy to %s\n", binaryPath)
		}
		return nil
	}

	if version, err := CurrentVersion(binaryPath); err == nil && version == config.DefaultCaddyVersion {
		return nil
	}

	tmpDir, err := os.MkdirTemp("", "dtsw-caddy-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	archivePath := filepath.Join(tmpDir, asset)
	if err := ioutil.DownloadToFile(ctx, url, archivePath); err != nil {
		return err
	}
	checksumsPath := filepath.Join(tmpDir, "checksums.txt")
	if err := ioutil.DownloadToFile(ctx, checksumsURL, checksumsPath); err != nil {
		return err
	}
	if err := verifyCaddyArchiveChecksum(archivePath, checksumsPath, asset); err != nil {
		return err
	}

	tmpBinary := filepath.Join(tmpDir, "caddy")
	if err := extractCaddyBinary(archivePath, tmpBinary); err != nil {
		return err
	}
	if err := os.Chmod(tmpBinary, 0o755); err != nil {
		return err
	}
	return ioutil.CopyFile(tmpBinary, binaryPath, 0o755)
}

func CurrentVersion(binary string) (string, error) {
	out, err := exec.Command(binary, "version").CombinedOutput()
	if err != nil {
		return "", err
	}
	re := regexp.MustCompile(`v[0-9]+\.[0-9]+\.[0-9]+`)
	match := re.FindString(string(out))
	if match == "" {
		return "", fmt.Errorf("unexpected caddy version output: %q", string(out))
	}
	return match, nil
}

func verifyCaddyArchiveChecksum(archivePath, checksumsPath, asset string) error {
	expected, err := caddyChecksumFromFile(checksumsPath, asset)
	if err != nil {
		return err
	}
	actual, err := sha256File(archivePath)
	if err != nil {
		return err
	}
	if !strings.EqualFold(expected, actual) {
		return fmt.Errorf("caddy archive checksum mismatch: expected %s got %s", expected, actual)
	}
	return nil
}

func caddyChecksumFromFile(path, asset string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		if fields[len(fields)-1] != asset {
			continue
		}
		return strings.ToLower(fields[0]), nil
	}
	return "", fmt.Errorf("failed to find checksum for %s", asset)
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

func extractCaddyBinary(archivePath, outPath string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer f.Close()
	gzr, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gzr.Close()
	tr := tar.NewReader(gzr)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if filepath.Base(header.Name) != "caddy" {
			continue
		}
		out, err := os.OpenFile(outPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
		if err != nil {
			return err
		}
		if _, err := io.Copy(out, tr); err != nil {
			out.Close()
			return err
		}
		return out.Close()
	}
	return fmt.Errorf("caddy binary not found in %s", archivePath)
}
