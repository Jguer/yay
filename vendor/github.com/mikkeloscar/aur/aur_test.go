package aur

import "testing"

// TestSearch test searching for packages by name and/or description
func TestSearch(t *testing.T) {
	rs, err := Search("linux")
	if err != nil {
		t.Error(err)
	}

	if len(rs) < 100 {
		t.Errorf("Expected more than 100 packages, got '%d'", len(rs))
	}

	rs, err = Search("li")
	if err.Error() != "Too many package results." {
		t.Errorf("Expected error 'Too many package results.', got '%s'", err.Error())
	}

	if len(rs) > 0 {
		t.Errorf("Expected no results, got '%d'", len(rs))
	}
}

// TestSearchByNameDesc test searching for packages package name and desc.
func TestSearchByNameDesc(t *testing.T) {
	rs, err := SearchByNameDesc("linux")
	if err != nil {
		t.Error(err)
	}

	if len(rs) < 100 {
		t.Errorf("Expected more than 100 packages, got '%d'", len(rs))
	}

	rs, err = Search("li")
	if err.Error() != "Too many package results." {
		t.Errorf("Expected error 'Too many package results.', got '%s'", err.Error())
	}

	if len(rs) > 0 {
		t.Errorf("Expected no results, got '%d'", len(rs))
	}
}

// TestSearchByMaintainer test searching for packages by maintainer
func TestSearchByMaintainer(t *testing.T) {
	rs, err := SearchByMaintainer("moscar")
	if err != nil {
		t.Error(err)
	}

	if len(rs) < 3 {
		t.Errorf("Expected more than 3 packages, got '%d'", len(rs))
	}
}

// TestInfo test getting info for multiple packages
func TestInfo(t *testing.T) {
	rs, err := Info([]string{"neovim-git", "linux-mainline"})
	if err != nil {
		t.Error(err)
	}

	if len(rs) != 2 {
		t.Errorf("Expected two packages, got %d", len(rs))
	}
}
