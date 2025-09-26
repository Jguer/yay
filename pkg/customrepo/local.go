package customrepo

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Jguer/yay/v12/pkg/settings"
)

// LocalRepository represents a local filesystem repository
type LocalRepository struct {
	name       string
	path       string
	searchable bool
	priority   int
}

// NewLocalRepository creates a new local repository
func NewLocalRepository(config settings.CustomRepo) (*LocalRepository, error) {
	if config.Type != string(TypeLocal) {
		return nil, fmt.Errorf("invalid repository type for local repository: %s", config.Type)
	}
	
	if config.Path == "" {
		return nil, fmt.Errorf("path is required for local repository")
	}
	
	// Expand environment variables in path
	expandedPath := os.ExpandEnv(config.Path)
	
	// Check if path exists
	if _, err := os.Stat(expandedPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("local repository path does not exist: %s", expandedPath)
	}
	
	return &LocalRepository{
		name:       config.Name,
		path:       expandedPath,
		searchable: config.Searchable,
		priority:   config.Priority,
	}, nil
}

func (r *LocalRepository) Name() string {
	return r.name
}

func (r *LocalRepository) Type() RepositoryType {
	return TypeLocal
}

func (r *LocalRepository) IsSearchable() bool {
	return r.searchable
}

func (r *LocalRepository) Priority() int {
	return r.priority
}

func (r *LocalRepository) Update(ctx context.Context) error {
	// Local repositories don't need updating
	return nil
}

func (r *LocalRepository) Search(ctx context.Context, query string) ([]PackageInfo, error) {
	var results []PackageInfo
	
	err := filepath.Walk(r.path, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		
		// Skip if not a directory
		if !info.IsDir() {
			return nil
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

func (r *LocalRepository) GetPackage(ctx context.Context, name string) (*PackageInfo, error) {
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

func (r *LocalRepository) GetPKGBUILDPath(ctx context.Context, name string) (string, error) {
	packagePath := filepath.Join(r.path, name)
	pkgbuildPath := filepath.Join(packagePath, "PKGBUILD")
	
	// Check if PKGBUILD exists
	if _, err := os.Stat(pkgbuildPath); os.IsNotExist(err) {
		return "", fmt.Errorf("package not found: %s", name)
	}
	
	return pkgbuildPath, nil
}

// parsePKGBUILD parses a PKGBUILD file to extract package information
func (r *LocalRepository) parsePKGBUILD(pkgbuildPath string) (*PackageInfo, error) {
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
		LastUpdated: time.Now(), // For local repos, use current time
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

