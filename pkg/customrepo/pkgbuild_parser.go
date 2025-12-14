package customrepo

import (
	"strings"
	"time"
)

// extractVariable extracts a simple variable value from PKGBUILD content
func extractVariable(content, varName string) string {
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, varName+"=") {
			value := strings.TrimPrefix(line, varName+"=")
			value = strings.Trim(value, `"'`)
			return value
		}
	}
	return ""
}

// extractArrayVariable extracts an array variable from PKGBUILD content
func extractArrayVariable(content, varName string) []string {
	lines := strings.Split(content, "\n")
	var inArray bool
	var values []string
	
	for _, line := range lines {
		line = strings.TrimSpace(line)
		
		if strings.HasPrefix(line, varName+"=(") {
			inArray = true
			// Check if array is on the same line
			if strings.HasSuffix(line, ")") {
				// Single line array - split by spaces and clean up
				value := strings.TrimPrefix(line, varName+"=(")
				value = strings.TrimSuffix(value, ")")
				value = strings.Trim(value, `"'`)
				if value != "" {
					// Split by spaces and clean up each element
					elements := strings.Fields(value)
					for _, elem := range elements {
						elem = strings.Trim(elem, `"'`)
						if elem != "" {
							values = append(values, elem)
						}
					}
				}
				break
			}
			continue
		}
		
		if inArray {
			if strings.HasSuffix(line, ")") {
				// End of array
				if line != ")" {
					value := strings.TrimSuffix(line, ")")
					value = strings.Trim(value, `"'`)
					if value != "" {
						values = append(values, value)
					}
				}
				break
			}
			
			// Skip empty lines and comments
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			
			// Skip lines that look like incomplete array declarations
			if strings.Contains(line, "=(") && !strings.HasSuffix(line, ")") {
				continue
			}
			
			// Array element
			value := strings.Trim(line, `"'`)
			if value != "" {
				values = append(values, value)
			}
		}
	}
	
	return values
}

// parsePKGBUILDContent parses PKGBUILD content to extract package information
func parsePKGBUILDContent(content string) (*PackageInfo, error) {
	// This is a simplified parser - in a real implementation,
	// you would use a proper PKGBUILD parser or shell execution
	// to extract the variables
	
	// Extract basic information using simple string parsing
	// This is a basic implementation - a real one would be more robust
	pkgInfo := &PackageInfo{
		LastUpdated: time.Now(), // For HTTP repos, use current time
	}
	
	// Extract pkgname
	if pkgname := extractVariable(content, "pkgname"); pkgname != "" {
		pkgInfo.Name = pkgname
	}
	
	// Extract pkgver
	if pkgver := extractVariable(content, "pkgver"); pkgver != "" {
		pkgInfo.Version = pkgver
	}
	
	// Extract pkgdesc
	if pkgdesc := extractVariable(content, "pkgdesc"); pkgdesc != "" {
		pkgInfo.Description = pkgdesc
	}
	
	// Extract provides
	if provides := extractArrayVariable(content, "provides"); len(provides) > 0 {
		pkgInfo.Provides = provides
	}
	
	// Extract depends
	if depends := extractArrayVariable(content, "depends"); len(depends) > 0 {
		// Filter out invalid dependencies that might be parsing errors
		var validDepends []string
		for _, dep := range depends {
			// Skip dependencies that look like parsing errors
			if strings.Contains(dep, "=") && !strings.Contains(dep, ">=") && !strings.Contains(dep, "<=") && !strings.Contains(dep, "!=") {
				continue
			}
			validDepends = append(validDepends, dep)
		}
		pkgInfo.Depends = validDepends
	}
	
	// Extract conflicts
	if conflicts := extractArrayVariable(content, "conflicts"); len(conflicts) > 0 {
		pkgInfo.Conflicts = conflicts
	}
	
	return pkgInfo, nil
}
