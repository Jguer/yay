package customrepo

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/Jguer/yay/v12/pkg/settings"
)

// GitRepository represents a git-based repository
type GitRepository struct {
	name       string
	url        string
	branch     string
	path       string
	searchable bool
	priority   int
	auth       *settings.RepoAuth
}

// NewGitRepository creates a new git repository
func NewGitRepository(config settings.CustomRepo, cacheDir string) (*GitRepository, error) {
	if config.Type != string(TypeGit) {
		return nil, fmt.Errorf("invalid repository type for git repository: %s", config.Type)
	}
	
	if config.URL == "" {
		return nil, fmt.Errorf("URL is required for git repository")
	}
	
	// Create local cache directory for this repository
	repoPath := filepath.Join(cacheDir, "custom-repos", config.Name)
	
	// Set default branch if not specified
	branch := config.Branch
	if branch == "" {
		branch = "main"
	}
	
	return &GitRepository{
		name:       config.Name,
		url:        config.URL,
		branch:     branch,
		path:       repoPath,
		searchable: config.Searchable,
		priority:   config.Priority,
		auth:       config.Auth,
	}, nil
}

func (r *GitRepository) Name() string {
	return r.name
}

func (r *GitRepository) Type() RepositoryType {
	return TypeGit
}

func (r *GitRepository) IsSearchable() bool {
	return r.searchable
}

func (r *GitRepository) Priority() int {
	return r.priority
}

func (r *GitRepository) Update(ctx context.Context) error {
	// Check if repository exists locally
	if _, err := os.Stat(filepath.Join(r.path, ".git")); os.IsNotExist(err) {
		// Clone repository
		return r.clone(ctx)
	}
	
	// Update existing repository
	return r.pull(ctx)
}

func (r *GitRepository) Search(ctx context.Context, query string) ([]PackageInfo, error) {
	// Ensure repository is up to date
	if err := r.Update(ctx); err != nil {
		return nil, err
	}
	
	var results []PackageInfo
	
	err := filepath.Walk(r.path, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		
		// Skip if not a directory
		if !info.IsDir() {
			return nil
		}
		
		// Skip .git directory
		if info.Name() == ".git" {
			return filepath.SkipDir
		}
		
		// Check if this directory contains a PKGBUILD
		pkgbuildPath := filepath.Join(path, "PKGBUILD")
		if _, err := os.Stat(pkgbuildPath); os.IsNotExist(err) {
			return nil
		}
		
		// Get package name from directory name
		packageName := filepath.Base(path)
		
		// Check if package name matches query
		if !strings.Contains(strings.ToLower(packageName), strings.ToLower(query)) {
			return nil
		}
		
		// Read PKGBUILD to get package info
		pkgInfo, err := r.parsePKGBUILD(pkgbuildPath)
		if err != nil {
			return nil // Skip packages with invalid PKGBUILDs
		}
		
		pkgInfo.Name = packageName
		pkgInfo.Source = r.name
		pkgInfo.Path = pkgbuildPath
		
		results = append(results, *pkgInfo)
		
		return nil
	})
	
	return results, err
}

func (r *GitRepository) GetPackage(ctx context.Context, name string) (*PackageInfo, error) {
	// Ensure repository is up to date
	if err := r.Update(ctx); err != nil {
		return nil, err
	}
	
	packagePath := filepath.Join(r.path, name)
	pkgbuildPath := filepath.Join(packagePath, "PKGBUILD")
	
	// Check if PKGBUILD exists
	if _, err := os.Stat(pkgbuildPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("package not found: %s", name)
	}
	
	pkgInfo, err := r.parsePKGBUILD(pkgbuildPath)
	if err != nil {
		return nil, err
	}
	
	pkgInfo.Name = name
	pkgInfo.Source = r.name
	pkgInfo.Path = pkgbuildPath
	
	return pkgInfo, nil
}

func (r *GitRepository) GetPKGBUILDPath(ctx context.Context, name string) (string, error) {
	// Ensure repository is up to date
	if err := r.Update(ctx); err != nil {
		return "", err
	}
	
	packagePath := filepath.Join(r.path, name)
	pkgbuildPath := filepath.Join(packagePath, "PKGBUILD")
	
	// Check if PKGBUILD exists
	if _, err := os.Stat(pkgbuildPath); os.IsNotExist(err) {
		return "", fmt.Errorf("package not found: %s", name)
	}
	
	return pkgbuildPath, nil
}

func (r *GitRepository) clone(ctx context.Context) error {
	// Create directory if it doesn't exist
	if err := os.MkdirAll(r.path, 0755); err != nil {
		return err
	}
	
	// Build git clone command
	args := []string{"clone", "--branch", r.branch, "--single-branch"}
	
	// Add authentication if configured
	if r.auth != nil {
		switch r.auth.Type {
		case "ssh_key":
			// SSH key authentication is handled by SSH agent or key files
			// No special arguments needed
		case "token":
			// Token authentication for HTTPS
			if r.auth.Token != "" {
				// Modify URL to include token
				r.url = strings.Replace(r.url, "https://", fmt.Sprintf("https://%s@", r.auth.Token), 1)
			}
		case "basic":
			// Basic authentication for HTTPS
			if r.auth.Username != "" && r.auth.Password != "" {
				r.url = strings.Replace(r.url, "https://", fmt.Sprintf("https://%s:%s@", r.auth.Username, r.auth.Password), 1)
			}
		}
	}
	
	args = append(args, r.url, r.path)
	
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = filepath.Dir(r.path)
	
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to clone repository: %s, output: %s", err, string(output))
	}
	
	return nil
}

func (r *GitRepository) pull(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "git", "pull", "origin", r.branch)
	cmd.Dir = r.path
	
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to pull repository: %s, output: %s", err, string(output))
	}
	
	return nil
}

// parsePKGBUILD parses a PKGBUILD file to extract package information
func (r *GitRepository) parsePKGBUILD(pkgbuildPath string) (*PackageInfo, error) {
	// This is a simplified parser - in a real implementation,
	// you would use a proper PKGBUILD parser or shell execution
	// to extract the variables
	
	content, err := os.ReadFile(pkgbuildPath)
	if err != nil {
		return nil, err
	}
	
	contentStr := string(content)
	
	// Extract basic information using simple string parsing
	// This is a basic implementation - a real one would be more robust
	pkgInfo := &PackageInfo{
		LastUpdated: time.Now(), // For git repos, we could get last commit time
	}
	
	// Extract pkgname
	if pkgname := extractVariable(contentStr, "pkgname"); pkgname != "" {
		pkgInfo.Name = pkgname
	}
	
	// Extract pkgver
	if pkgver := extractVariable(contentStr, "pkgver"); pkgver != "" {
		pkgInfo.Version = pkgver
	}
	
	// Extract pkgdesc
	if pkgdesc := extractVariable(contentStr, "pkgdesc"); pkgdesc != "" {
		pkgInfo.Description = pkgdesc
	}
	
	// Extract provides
	if provides := extractArrayVariable(contentStr, "provides"); len(provides) > 0 {
		pkgInfo.Provides = provides
	}
	
	// Extract depends
	if depends := extractArrayVariable(contentStr, "depends"); len(depends) > 0 {
		pkgInfo.Depends = depends
	}
	
	// Extract conflicts
	if conflicts := extractArrayVariable(contentStr, "conflicts"); len(conflicts) > 0 {
		pkgInfo.Conflicts = conflicts
	}
	
	return pkgInfo, nil
}

