package golang

import (
	"fmt"
	"regexp"
	"strings"
)

type Fragment struct {
	Name string
	SQL  string
}

type FragmentRegistry struct {
	fragments map[string]Fragment
}

func NewFragmentRegistry() *FragmentRegistry {
	return &FragmentRegistry{
		fragments: make(map[string]Fragment),
	}
}

func (fr *FragmentRegistry) AddFragment(name, sql string) {
	fr.fragments[name] = Fragment{
		Name: name,
		SQL:  strings.TrimSpace(sql),
	}
}

func (fr *FragmentRegistry) GetFragment(name string) (Fragment, bool) {
	fragment, exists := fr.fragments[name]
	return fragment, exists
}

func (fr *FragmentRegistry) SubstituteFragments(sql string) (string, error) {
	fragmentRefRegex := regexp.MustCompile(`\{\{fragment:([a-zA-Z_][a-zA-Z0-9_]*)\}\}`)
	
	result := fragmentRefRegex.ReplaceAllStringFunc(sql, func(match string) string {
		matches := fragmentRefRegex.FindStringSubmatch(match)
		if len(matches) < 2 {
			return match
		}
		
		fragmentName := matches[1]
		fragment, exists := fr.GetFragment(fragmentName)
		if !exists {
			return match
		}
		
		return fragment.SQL
	})
	
	// Check for any remaining unresolved fragment references
	if fragmentRefRegex.MatchString(result) {
		matches := fragmentRefRegex.FindAllStringSubmatch(result, -1)
		if len(matches) > 0 {
			return "", fmt.Errorf("undefined fragment: %s", matches[0][1])
		}
	}
	
	return result, nil
}

func ParseFragmentsFromSQL(sqlContent string) *FragmentRegistry {
	registry := NewFragmentRegistry()
	
	fragmentDefRegex := regexp.MustCompile(`(?m)^\s*--\s*fragment:\s*([a-zA-Z_][a-zA-Z0-9_]*)\s*$`)
	lines := strings.Split(sqlContent, "\n")
	
	for i, line := range lines {
		matches := fragmentDefRegex.FindStringSubmatch(line)
		if len(matches) >= 2 {
			fragmentName := strings.TrimSpace(matches[1])
			
			// Collect SQL lines until we hit another comment or query
			var sqlLines []string
			for j := i + 1; j < len(lines); j++ {
				nextLine := strings.TrimSpace(lines[j])
				
				// Stop at empty lines, comments, or query definitions
				if nextLine == "" || 
				   strings.HasPrefix(nextLine, "--") ||
				   strings.Contains(nextLine, ":one") ||
				   strings.Contains(nextLine, ":many") ||
				   strings.Contains(nextLine, ":exec") {
					break
				}
				
				sqlLines = append(sqlLines, lines[j])
			}
			
			if len(sqlLines) > 0 {
				fragmentSQL := strings.Join(sqlLines, "\n")
				registry.AddFragment(fragmentName, fragmentSQL)
			}
		}
	}
	
	return registry
}