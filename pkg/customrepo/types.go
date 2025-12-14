package customrepo

import (
	"context"
	"time"

	"github.com/Jguer/yay/v12/pkg/settings"
)

// RepositoryType represents the type of custom repository
type RepositoryType string

const (
	TypeLocal RepositoryType = "local"
	TypeGit   RepositoryType = "git"
	TypeHTTP  RepositoryType = "http"
)

// PackageInfo represents package information from custom repositories
type PackageInfo struct {
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Version     string    `json:"version"`
	Source      string    `json:"source"`      // Repository name
	Path        string    `json:"path"`        // Local path to PKGBUILD
	LastUpdated time.Time `json:"lastUpdated"`
	Provides    []string  `json:"provides,omitempty"`
	Depends     []string  `json:"depends,omitempty"`
	Conflicts   []string  `json:"conflicts,omitempty"`
}

// Repository interface defines methods for custom repositories
type Repository interface {
	// Name returns the repository name
	Name() string
	
	// Type returns the repository type
	Type() RepositoryType
	
	// Search searches for packages matching the query
	Search(ctx context.Context, query string) ([]PackageInfo, error)
	
	// GetPackage retrieves a specific package by name
	GetPackage(ctx context.Context, name string) (*PackageInfo, error)
	
	// Update updates the repository (for git/http types)
	Update(ctx context.Context) error
	
	// IsSearchable returns whether this repository should be included in search
	IsSearchable() bool
	
	// Priority returns the search priority (lower = higher priority)
	Priority() int
	
	// GetPKGBUILDPath returns the local path to a package's PKGBUILD
	GetPKGBUILDPath(ctx context.Context, name string) (string, error)
}

// Manager manages custom repositories
type Manager struct {
	repos map[string]Repository
	config *settings.Configuration
}

// NewManager creates a new repository manager
func NewManager(config *settings.Configuration) *Manager {
	return &Manager{
		repos:  make(map[string]Repository),
		config: config,
	}
}

// AddRepository adds a custom repository
func (m *Manager) AddRepository(repo Repository) {
	m.repos[repo.Name()] = repo
}

// GetRepository retrieves a repository by name
func (m *Manager) GetRepository(name string) (Repository, bool) {
	repo, exists := m.repos[name]
	return repo, exists
}

// ListRepositories returns all configured repositories
func (m *Manager) ListRepositories() []Repository {
	repos := make([]Repository, 0, len(m.repos))
	for _, repo := range m.repos {
		repos = append(repos, repo)
	}
	return repos
}

// SearchAll searches across all searchable repositories
func (m *Manager) SearchAll(ctx context.Context, query string) ([]PackageInfo, error) {
	var allResults []PackageInfo
	
	for _, repo := range m.repos {
		if !repo.IsSearchable() {
			continue
		}
		
		results, err := repo.Search(ctx, query)
		if err != nil {
			// Log error but continue with other repositories
			continue
		}
		
		allResults = append(allResults, results...)
	}
	
	// Sort by priority and then by name
	// This would be implemented in a separate sorting function
	
	return allResults, nil
}

// UpdateAll updates all repositories that support updating
func (m *Manager) UpdateAll(ctx context.Context) error {
	for _, repo := range m.repos {
		if repo.Type() == TypeLocal {
			continue // Local repositories don't need updating
		}
		
		if err := repo.Update(ctx); err != nil {
			// Log error but continue with other repositories
			continue
		}
	}
	
	return nil
}