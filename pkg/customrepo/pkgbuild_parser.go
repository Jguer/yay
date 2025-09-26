package customrepo

import (
	"strings"
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
			
			// Array element
			value := strings.Trim(line, `"'`)
			if value != "" {
				values = append(values, value)
			}
		}
	}
	
	return values
}
