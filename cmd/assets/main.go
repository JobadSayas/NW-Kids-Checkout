package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	staticPrefix = "/static/"
)

var assetRegex = regexp.MustCompile(`(href|src)\s*=\s*"([^"]+)"`)

func main() {
	environment := strings.ToLower(strings.TrimSpace(os.Getenv("ENVIRONMENT")))
	if environment == "dev" {
		fmt.Println("Skipping asset versioning in dev environment.")
		return
	}

	root, err := os.Getwd()
	if err != nil {
		fatal(err)
	}

	staticDir := filepath.Join(root, "internal", "web", "static")
	pagesDir := filepath.Join(staticDir, "pages")

	htmlFiles, err := listHTMLFiles(pagesDir)
	if err != nil {
		fatal(err)
	}

	assetURLs := collectAssetURLs(htmlFiles)
	versionMap, err := buildVersionMap(staticDir, assetURLs)
	if err != nil {
		fatal(err)
	}

	updated, err := updateHTMLFiles(htmlFiles, versionMap)
	if err != nil {
		fatal(err)
	}

	if len(updated) == 0 {
		fmt.Println("No asset versions updated.")
		return
	}

	fmt.Println("Updated asset versions in:")
	for _, file := range updated {
		fmt.Printf("- %s\n", file)
	}
}

func listHTMLFiles(root string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if strings.HasSuffix(d.Name(), ".html") {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}

func collectAssetURLs(htmlFiles []string) []string {
	set := map[string]struct{}{}
	for _, file := range htmlFiles {
		contents, err := os.ReadFile(file)
		if err != nil {
			fatal(err)
		}
		matches := assetRegex.FindAllStringSubmatch(string(contents), -1)
		for _, match := range matches {
			if len(match) < 3 {
				continue
			}
			assetURL := match[2]
			if !isVersionedAsset(assetURL) {
				continue
			}
			baseURL := stripQuery(assetURL)
			set[baseURL] = struct{}{}
		}
	}

	urls := make([]string, 0, len(set))
	for key := range set {
		urls = append(urls, key)
	}
	return urls
}

func buildVersionMap(staticDir string, assetURLs []string) (map[string]string, error) {
	versionMap := make(map[string]string, len(assetURLs))
	for _, assetURL := range assetURLs {
		assetPath := assetPathFromURL(staticDir, assetURL)
		hash, err := fileHash(assetPath)
		if err != nil {
			return nil, err
		}
		versionMap[assetURL] = hash
	}
	return versionMap, nil
}

func updateHTMLFiles(htmlFiles []string, versionMap map[string]string) ([]string, error) {
	var updated []string
	for _, file := range htmlFiles {
		contents, err := os.ReadFile(file)
		if err != nil {
			return nil, err
		}
		original := string(contents)
		modified := assetRegex.ReplaceAllStringFunc(original, func(match string) string {
			parts := assetRegex.FindStringSubmatch(match)
			if len(parts) < 3 {
				return match
			}
			attr := parts[1]
			assetURL := parts[2]
			if !isVersionedAsset(assetURL) {
				return match
			}
			baseURL := stripQuery(assetURL)
			version, ok := versionMap[baseURL]
			if !ok {
				return match
			}
			versioned := withVersion(baseURL, version)
			return fmt.Sprintf("%s=\"%s\"", attr, versioned)
		})

		if modified != original {
			if err := os.WriteFile(file, []byte(modified), 0644); err != nil {
				return nil, err
			}
			updated = append(updated, file)
		}
	}
	return updated, nil
}

func isVersionedAsset(assetURL string) bool {
	if !strings.HasPrefix(assetURL, staticPrefix) {
		return false
	}
	return strings.Contains(assetURL, ".css") || strings.Contains(assetURL, ".js")
}

func stripQuery(assetURL string) string {
	if idx := strings.Index(assetURL, "?"); idx >= 0 {
		return assetURL[:idx]
	}
	return assetURL
}

func assetPathFromURL(staticDir, assetURL string) string {
	relative := strings.TrimPrefix(assetURL, staticPrefix)
	return filepath.Join(staticDir, filepath.FromSlash(relative))
}

func fileHash(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hasher.Sum(nil))[:12], nil
}

func withVersion(assetURL, version string) string {
	parsed, err := url.Parse(assetURL)
	if err != nil {
		return assetURL
	}
	query := parsed.Query()
	query.Set("v", version)
	parsed.RawQuery = query.Encode()
	return parsed.String()
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err.Error())
	os.Exit(1)
}
