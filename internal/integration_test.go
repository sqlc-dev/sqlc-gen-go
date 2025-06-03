package golang

import (
	"testing"

	"github.com/sqlc-dev/plugin-sdk-go/plugin"
	"github.com/sqlc-dev/sqlc-gen-go/internal/opts"
)

func TestBuildQueriesWithFragments(t *testing.T) {
	// Create a mock request with queries that use fragments
	req := &plugin.GenerateRequest{
		Catalog: &plugin.Catalog{
			DefaultSchema: "public",
		},
		Settings: &plugin.Settings{
			Engine: "postgresql",
		},
		Queries: []*plugin.Query{
			{
				Name:     "GetActiveAuthor",
				Cmd:      ":one", 
				Filename: "query.sql",
				Text: `{{fragment:author_select}}
{{fragment:active_filter}}
AND id = $1
LIMIT 1;`,
				Params: []*plugin.Parameter{
					{
						Number: 1,
						Column: &plugin.Column{
							Name: "id",
							Type: &plugin.Identifier{Name: "int4"},
						},
					},
				},
				Columns: []*plugin.Column{
					{
						Name: "id",
						Type: &plugin.Identifier{Name: "int4"},
					},
					{
						Name: "name", 
						Type: &plugin.Identifier{Name: "text"},
					},
					{
						Name: "bio",
						Type: &plugin.Identifier{Name: "text"},
					},
				},
			},
		},
	}

	// Add some fragment definitions to simulate how they would appear in SQL
	fragmentQuery := &plugin.Query{
		Name:     "",  // Fragments don't have names
		Cmd:      "",
		Filename: "query.sql",
		Text:     "-- fragment: author_select\nSELECT id, name, bio FROM authors\n\n-- fragment: active_filter\nWHERE deleted_at IS NULL",
	}
	req.Queries = append([]*plugin.Query{fragmentQuery}, req.Queries...)

	options := &opts.Options{
		Package: "test",
		QueryParameterLimit: func() *int32 { v := int32(1); return &v }(),
		Initialisms: func() *[]string { v := []string{"id"}; return &v }(),
		InitialismsMap: map[string]struct{}{"id": {}},
	}

	queries, err := buildQueries(req, options, []Struct{})
	if err != nil {
		t.Fatalf("buildQueries failed: %v", err)
	}

	if len(queries) != 1 {
		t.Fatalf("Expected 1 query, got %d", len(queries))
	}

	query := queries[0]
	expected := `SELECT id, name, bio FROM authors
WHERE deleted_at IS NULL
AND id = $1
LIMIT 1;`

	if query.SQL != expected {
		t.Errorf("Expected SQL:\n%s\n\nGot:\n%s", expected, query.SQL)
	}

	if query.MethodName != "GetActiveAuthor" {
		t.Errorf("Expected MethodName 'GetActiveAuthor', got '%s'", query.MethodName)
	}
}