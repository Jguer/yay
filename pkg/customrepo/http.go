package customrepo

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Jguer/yay/v12/pkg/settings"
)

// HTTPRepository represents an HTTP-based repository
type HTTPRepository struct {
	name       string
	url        string
	path       string
	searchable bool
	priority   int
	auth       *settings.RepoAuth
	client     *http.Client
}

// NewHTTPRepository creates a new HTTP repository
func NewHTTPRepository(config settings.CustomRepo, cacheDir string) (*HTTPRepository, error) {
	if config.Type != string(TypeHTTP) {
		return nil, fmt.Errorf("invalid repository type for HTTP repository: %s", config.Type)
	}
	
	if config.URL == "" {
		return nil, fmt.Errorf("URL is required for HTTP repository")
	}
	
	// Create local cache directory for this repository
	repoPath := filepath.Join(cacheDir, "custom-repos", config.Name)
	
	return &HTTPRepository{
		name:       config.Name,
		url:        config.URL,
		path:       repoPath,
		searchable: config.Searchable,
		priority:   config.Priority,
		auth:       config.Auth,
		client:     &http.Client{Timeout: 30 * time.Second},
	}, nil
}

func (r *HTTPRepository) Name() string {
	return r.name
}

func (r *HTTPRepository) Type() RepositoryType {
	return TypeHTTP
}

func (r *HTTPRepository) IsSearchable() bool {
	return r.searchable
}

func (r *HTTPRepository) Priority() int {
	return r.priority
}

func (r *HTTPRepository) Update(ctx context.Context) error {
	// Create directory if it doesn't exist
	if err := os.MkdirAll(r.path, 0755); err != nil {
		return err
	}
	
	// Try to fetch packages.json first for metadata
	packagesURL := strings.TrimSuffix(r.url, "/") + "/packages.json"
	req, err := http.NewRequestWithContext(ctx, "GET", packagesURL, nil)
	if err != nil {
		return err
	}
	
	// Add authentication if configured
	if r.auth != nil {
		switch r.auth.Type {
		case "token":
			req.Header.Set("Authorization", "Bearer "+r.auth.Token)
		case "basic":
			req.SetBasicAuth(r.auth.Username, r.auth.Password)
		}
	}
	
	resp, err := r.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode == http.StatusOK {
		// Parse packages.json if available
		var packagesData struct {
			Packages []PackageInfo `json:"packages"`
		}
		
		if err := json.NewDecoder(resp.Body).Decode(&packagesData); err == nil {
			// Save packages metadata locally
			metadataPath := filepath.Join(r.path, "packages.json")
			file, err := os.Create(metadataPath)
			if err == nil {
				json.NewEncoder(file).Encode(packagesData)
				file.Close()
			}
		}
	}
	
	return nil
}

func (r *HTTPRepository) Search(ctx context.Context, query string) ([]PackageInfo, error) {
	// Try to use cached packages.json first
	metadataPath := filepath.Join(r.path, "packages.json")
	if _, err := os.Stat(metadataPath); err == nil {
		return r.searchFromCache(query)
	}
	
	// Fallback to directory listing
	return r.searchFromDirectory(ctx, query)
}

func (r *HTTPRepository) GetPackage(ctx context.Context, name string) (*PackageInfo, error) {
	// Try to use cached packages.json first
	metadataPath := filepath.Join(r.path, "packages.json")
	if _, err := os.Stat(metadataPath); err == nil {
		return r.getPackageFromCache(name)
	}
	
	// Fallback to directory listing
	return r.getPackageFromDirectory(ctx, name)
}

func (r *HTTPRepository) GetPKGBUILDPath(ctx context.Context, name string) (string, error) {
	// Download PKGBUILD to local cache
	localPath := filepath.Join(r.path, name, "PKGBUILD")
	
	// Check if already cached
	if _, err := os.Stat(localPath); err == nil {
		return localPath, nil
	}
	
	// Download PKGBUILD
	pkgbuildURL := strings.TrimSuffix(r.url, "/") + "/" + name + "/PKGBUILD"
	req, err := http.NewRequestWithContext(ctx, "GET", pkgbuildURL, nil)
	if err != nil {
		return "", err
	}
	
	// Add authentication if configured
	if r.auth != nil {
		switch r.auth.Type {
		case "token":
			req.Header.Set("Authorization", "Bearer "+r.auth.Token)
		case "basic":
			req.SetBasicAuth(r.auth.Username, r.auth.Password)
		}
	}
	
	resp, err := r.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("package not found: %s", name)
	}
	
	// Create directory for package
	if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
		return "", err
	}
	
	// Save PKGBUILD locally
	file, err := os.Create(localPath)
	if err != nil {
		return "", err
	}
	defer file.Close()
	
	_, err = io.Copy(file, resp.Body)
	if err != nil {
		return "", err
	}
	
	// Also download .SRCINFO if available
	srcinfoPath := filepath.Join(r.path, name, ".SRCINFO")
	srcinfoURL := strings.TrimSuffix(r.url, "/") + "/" + name + "/.SRCINFO"
	
	srcinfoReq, err := http.NewRequestWithContext(ctx, "GET", srcinfoURL, nil)
	if err == nil {
		// Add authentication if configured
		if r.auth != nil {
			switch r.auth.Type {
			case "token":
				srcinfoReq.Header.Set("Authorization", "Bearer "+r.auth.Token)
			case "basic":
				srcinfoReq.SetBasicAuth(r.auth.Username, r.auth.Password)
			}
		}
		
		srcinfoResp, err := r.client.Do(srcinfoReq)
		if err == nil && srcinfoResp.StatusCode == http.StatusOK {
			srcinfoFile, err := os.Create(srcinfoPath)
			if err == nil {
				io.Copy(srcinfoFile, srcinfoResp.Body)
				srcinfoFile.Close()
			}
			srcinfoResp.Body.Close()
		} else if err != nil {
			// Log error for debugging
			fmt.Printf("Failed to download .SRCINFO: %v\n", err)
		} else {
			fmt.Printf("Failed to download .SRCINFO: status %d\n", srcinfoResp.StatusCode)
		}
	} else {
		fmt.Printf("Failed to create .SRCINFO request: %v\n", err)
	}
	
	return localPath, nil
}

func (r *HTTPRepository) searchFromCache(query string) ([]PackageInfo, error) {
	metadataPath := filepath.Join(r.path, "packages.json")
	file, err := os.Open(metadataPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	
	var packagesData struct {
		Packages []PackageInfo `json:"packages"`
	}
	
	if err := json.NewDecoder(file).Decode(&packagesData); err != nil {
		return nil, err
	}
	
	var results []PackageInfo
	for _, pkg := range packagesData.Packages {
		if strings.Contains(strings.ToLower(pkg.Name), strings.ToLower(query)) ||
		   strings.Contains(strings.ToLower(pkg.Description), strings.ToLower(query)) {
			pkg.Source = r.name
			results = append(results, pkg)
		}
	}
	
	return results, nil
}

func (r *HTTPRepository) getPackageFromCache(name string) (*PackageInfo, error) {
	metadataPath := filepath.Join(r.path, "packages.json")
	file, err := os.Open(metadataPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	
	var packagesData struct {
		Packages []PackageInfo `json:"packages"`
	}
	
	if err := json.NewDecoder(file).Decode(&packagesData); err != nil {
		return nil, err
	}
	
	for _, pkg := range packagesData.Packages {
		if pkg.Name == name {
			pkg.Source = r.name
			return &pkg, nil
		}
	}
	
	return nil, fmt.Errorf("package not found: %s", name)
}

func (r *HTTPRepository) searchFromDirectory(ctx context.Context, query string) ([]PackageInfo, error) {
	// Get directory listing
	req, err := http.NewRequestWithContext(ctx, "GET", r.url, nil)
	if err != nil {
		return nil, err
	}
	
	// Add authentication if configured
	if r.auth != nil {
		switch r.auth.Type {
		case "token":
			req.Header.Set("Authorization", "Bearer "+r.auth.Token)
		case "basic":
			req.SetBasicAuth(r.auth.Username, r.auth.Password)
		}
	}
	
	resp, err := r.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get directory listing: %s", resp.Status)
	}
	
	// Read directory listing HTML
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	
	// Parse HTML to find package directories
	// Look for links that end with "/"
	html := string(body)
	var packages []PackageInfo
	
	// Simple regex-like parsing for directory links
	lines := strings.Split(html, "\n")
	for _, line := range lines {
		if strings.Contains(line, "href=") && strings.Contains(line, "/") {
			// Extract directory name from href
			start := strings.Index(line, "href=\"")
			if start != -1 {
				start += 6 // len("href=\"")
				end := strings.Index(line[start:], "\"")
				if end != -1 {
					dirName := line[start : start+end]
					if strings.HasSuffix(dirName, "/") && dirName != "../" && dirName != "./" {
						// Remove trailing slash
						dirName = strings.TrimSuffix(dirName, "/")
						
						// Check if this directory contains a PKGBUILD
						pkgInfo, err := r.getPackageFromDirectory(ctx, dirName)
						if err == nil {
							// Check if package name matches query
							if strings.Contains(strings.ToLower(pkgInfo.Name), strings.ToLower(query)) ||
							   strings.Contains(strings.ToLower(pkgInfo.Description), strings.ToLower(query)) {
								packages = append(packages, *pkgInfo)
							}
						}
					}
				}
			}
		}
	}
	
	return packages, nil
}

func (r *HTTPRepository) getPackageFromDirectory(ctx context.Context, name string) (*PackageInfo, error) {
	// Download PKGBUILD from the package directory
	pkgbuildURL := r.url + "/" + name + "/PKGBUILD"
	
	req, err := http.NewRequestWithContext(ctx, "GET", pkgbuildURL, nil)
	if err != nil {
		return nil, err
	}
	
	// Add authentication if configured
	if r.auth != nil {
		switch r.auth.Type {
		case "token":
			req.Header.Set("Authorization", "Bearer "+r.auth.Token)
		case "basic":
			req.SetBasicAuth(r.auth.Username, r.auth.Password)
		}
	}
	
	resp, err := r.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("package not found: %s", name)
	}
	
	// Read PKGBUILD content
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	
	// Parse PKGBUILD to extract package info
	pkgInfo, err := parsePKGBUILDContent(string(body))
	if err != nil {
		return nil, fmt.Errorf("failed to parse PKGBUILD: %w", err)
	}
	
	// Set source and path
	pkgInfo.Source = r.name
	pkgInfo.Path = filepath.Join(r.path, name, "PKGBUILD")
	
	return pkgInfo, nil
}
