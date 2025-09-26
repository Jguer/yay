package customrepo

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/Jguer/yay/v12/pkg/settings"
)

// RepositoryFactory creates repository instances from configuration
type RepositoryFactory struct {
	cacheDir string
}

// NewRepositoryFactory creates a new repository factory
func NewRepositoryFactory(cacheDir string) *RepositoryFactory {
	return &RepositoryFactory{
		cacheDir: cacheDir,
	}
}

// CreateRepository creates a repository instance from configuration
func (f *RepositoryFactory) CreateRepository(config settings.CustomRepo) (Repository, error) {
	switch config.Type {
	case string(TypeLocal):
		return NewLocalRepository(config)
	case string(TypeGit):
		return NewGitRepository(config, f.cacheDir)
	case string(TypeHTTP):
		return NewHTTPRepository(config, f.cacheDir)
	default:
		return nil, fmt.Errorf("unsupported repository type: %s", config.Type)
	}
}

// CreateManagerFromConfig creates a repository manager from configuration
func (f *RepositoryFactory) CreateManagerFromConfig(config *settings.Configuration) (*Manager, error) {
	manager := NewManager(config)
	
	for _, repoConfig := range config.CustomRepos {
		repo, err := f.CreateRepository(repoConfig)
		if err != nil {
			// Log error but continue with other repositories
			continue
		}
		
		manager.AddRepository(repo)
	}
	
	return manager, nil
}

// ValidateRepositoryConfig validates a repository configuration
func (f *RepositoryFactory) ValidateRepositoryConfig(config settings.CustomRepo) error {
	if config.Name == "" {
		return fmt.Errorf("repository name is required")
	}
	
	if config.Type == "" {
		return fmt.Errorf("repository type is required")
	}
	
	switch config.Type {
	case string(TypeLocal):
		if config.Path == "" {
			return fmt.Errorf("path is required for local repository")
		}
		// Expand environment variables
		expandedPath := os.ExpandEnv(config.Path)
		if _, err := os.Stat(expandedPath); os.IsNotExist(err) {
			return fmt.Errorf("local repository path does not exist: %s", expandedPath)
		}
	case string(TypeGit):
		if config.URL == "" {
			return fmt.Errorf("URL is required for git repository")
		}
	case string(TypeHTTP):
		if config.URL == "" {
			return fmt.Errorf("URL is required for HTTP repository")
		}
	default:
		return fmt.Errorf("unsupported repository type: %s", config.Type)
	}
	
	return nil
}

// GetDefaultCacheDir returns the default cache directory for custom repositories
func GetDefaultCacheDir() (string, error) {
	// Use the same cache directory as yay
	if cacheHome := os.Getenv("XDG_CACHE_HOME"); cacheHome != "" {
		cacheDir := filepath.Join(cacheHome, "yay")
		if err := os.MkdirAll(cacheDir, 0755); err == nil {
			return cacheDir, nil
		}
	}
	
	if home := os.Getenv("HOME"); home != "" {
		cacheDir := filepath.Join(home, ".cache", "yay")
		if err := os.MkdirAll(cacheDir, 0755); err == nil {
			return cacheDir, nil
		}
	}
	
	// Fallback to temp directory
	tmpDir := filepath.Join(os.TempDir(), "yay")
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		return "", err
	}
	
	return tmpDir, nil
}
