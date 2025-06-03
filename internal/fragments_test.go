package golang

import (
	"testing"
)

func TestParseFragmentsFromSQL(t *testing.T) {
	sqlContent := `-- fragment: author_select
SELECT id, name, bio FROM authors

-- fragment: active_filter
WHERE deleted_at IS NULL

-- name: GetActiveAuthor :one
{{fragment:author_select}}
{{fragment:active_filter}}
AND id = $1
LIMIT 1;`

	registry := ParseFragmentsFromSQL(sqlContent)

	// Test that fragments were parsed correctly
	authorSelect, exists := registry.GetFragment("author_select")
	if !exists {
		t.Error("Expected author_select fragment to exist")
	}
	if authorSelect.SQL != "SELECT id, name, bio FROM authors" {
		t.Errorf("Expected 'SELECT id, name, bio FROM authors', got '%s'", authorSelect.SQL)
	}

	activeFilter, exists := registry.GetFragment("active_filter")
	if !exists {
		t.Error("Expected active_filter fragment to exist")
	}
	if activeFilter.SQL != "WHERE deleted_at IS NULL" {
		t.Errorf("Expected 'WHERE deleted_at IS NULL', got '%s'", activeFilter.SQL)
	}
}

func TestFragmentSubstitution(t *testing.T) {
	registry := NewFragmentRegistry()
	registry.AddFragment("author_select", "SELECT id, name, bio FROM authors")
	registry.AddFragment("active_filter", "WHERE deleted_at IS NULL")

	sql := `{{fragment:author_select}}
{{fragment:active_filter}}
AND id = $1
LIMIT 1;`

	result, err := registry.SubstituteFragments(sql)
	if err != nil {
		t.Fatalf("Fragment substitution failed: %v", err)
	}

	expected := `SELECT id, name, bio FROM authors
WHERE deleted_at IS NULL
AND id = $1
LIMIT 1;`

	if result != expected {
		t.Errorf("Expected:\n%s\n\nGot:\n%s", expected, result)
	}
}

func TestUndefinedFragmentError(t *testing.T) {
	registry := NewFragmentRegistry()
	
	sql := `SELECT * FROM users {{fragment:undefined_fragment}}`
	
	_, err := registry.SubstituteFragments(sql)
	if err == nil {
		t.Error("Expected error for undefined fragment, but got none")
	}
	
	if err.Error() != "undefined fragment: undefined_fragment" {
		t.Errorf("Expected 'undefined fragment: undefined_fragment', got '%s'", err.Error())
	}
}