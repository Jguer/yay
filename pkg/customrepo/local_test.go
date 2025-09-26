package customrepo

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Jguer/yay/v12/pkg/settings"
)

func TestLocalRepository(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "yay-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a test package directory
	pkgDir := filepath.Join(tmpDir, "test-package")
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create a simple PKGBUILD
	pkgbuildContent := `pkgname=test-package
pkgver=1.0.0
pkgdesc=A test package
provides=('test')
depends=('bash')
conflicts=('old-test')
`
	pkgbuildPath := filepath.Join(pkgDir, "PKGBUILD")
	if err := os.WriteFile(pkgbuildPath, []byte(pkgbuildContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Create repository configuration
	config := settings.CustomRepo{
		Name:       "test-repo",
		Type:       "local",
		Path:       tmpDir,
		Searchable: true,
		Priority:   1,
	}

	// Create repository
	repo, err := NewLocalRepository(config)
	if err != nil {
		t.Fatal(err)
	}

	// Test basic properties
	if repo.Name() != "test-repo" {
		t.Errorf("Expected name 'test-repo', got '%s'", repo.Name())
	}

	if repo.Type() != TypeLocal {
		t.Errorf("Expected type 'local', got '%s'", repo.Type())
	}

	if !repo.IsSearchable() {
		t.Error("Expected repository to be searchable")
	}

	if repo.Priority() != 1 {
		t.Errorf("Expected priority 1, got %d", repo.Priority())
	}

	// Test search
	ctx := context.Background()
	results, err := repo.Search(ctx, "test")
	if err != nil {
		t.Fatal(err)
	}

	if len(results) != 1 {
		t.Errorf("Expected 1 search result, got %d", len(results))
	}

	if results[0].Name != "test-package" {
		t.Errorf("Expected package name 'test-package', got '%s'", results[0].Name)
	}

	if results[0].Source != "test-repo" {
		t.Errorf("Expected source 'test-repo', got '%s'", results[0].Source)
	}

	// Test get package
	pkg, err := repo.GetPackage(ctx, "test-package")
	if err != nil {
		t.Fatal(err)
	}

	if pkg.Name != "test-package" {
		t.Errorf("Expected package name 'test-package', got '%s'", pkg.Name)
	}

	// Test get PKGBUILD path
	pkgbuildPathResult, err := repo.GetPKGBUILDPath(ctx, "test-package")
	if err != nil {
		t.Fatal(err)
	}

	if pkgbuildPathResult != pkgbuildPath {
		t.Errorf("Expected PKGBUILD path '%s', got '%s'", pkgbuildPath, pkgbuildPathResult)
	}
}
