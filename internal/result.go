package golang

import (
	"bufio"
	"fmt"
	"github.com/sqlc-dev/sqlc-gen-go/internal/opts"
	"regexp"
	"sort"
	"strings"

	"github.com/sqlc-dev/plugin-sdk-go/metadata"
	"github.com/sqlc-dev/plugin-sdk-go/plugin"
	"github.com/sqlc-dev/plugin-sdk-go/sdk"
	"github.com/sqlc-dev/sqlc-gen-go/internal/inflection"
)

var queryHasLimitRegexp = regexp.MustCompile(`(?i)\s+limit\s+`)
var queryHasOffsetRegexp = regexp.MustCompile(`(?i)\s+offset\s+`)

var regexpOrderBy = regexp.MustCompile(`(?i)\s+order\s+by\s`)

func buildEnums(req *plugin.GenerateRequest, options *opts.Options) []Enum {
	var enums []Enum
	for _, schema := range req.Catalog.Schemas {
		if schema.Name == "pg_catalog" || schema.Name == "information_schema" {
			continue
		}
		for _, enum := range schema.Enums {
			var enumName string
			if schema.Name == req.Catalog.DefaultSchema {
				enumName = enum.Name
			} else {
				enumName = schema.Name + "_" + enum.Name
			}

			e := Enum{
				Name:      StructName(enumName, options),
				Comment:   enum.Comment,
				NameTags:  map[string]string{},
				ValidTags: map[string]string{},
			}
			if options.EmitJsonTags {
				e.NameTags["json"] = JSONTagName(enumName, options)
				e.ValidTags["json"] = JSONTagName("valid", options)
			}

			seen := make(map[string]struct{}, len(enum.Vals))
			for i, v := range enum.Vals {
				value := EnumReplace(v)
				if _, found := seen[value]; found || value == "" {
					value = fmt.Sprintf("value_%d", i)
				}
				e.Constants = append(
					e.Constants, Constant{
						Name:  StructName(enumName+"_"+value, options),
						Value: v,
						Type:  e.Name,
					},
				)
				seen[value] = struct{}{}
			}
			enums = append(enums, e)
		}
	}
	if len(enums) > 0 {
		sort.Slice(enums, func(i, j int) bool { return enums[i].Name < enums[j].Name })
	}
	return enums
}

func buildStructs(req *plugin.GenerateRequest, options *opts.Options) []Struct {
	var structs []Struct
	for _, schema := range req.Catalog.Schemas {
		if schema.Name == "pg_catalog" || schema.Name == "information_schema" {
			continue
		}
		for _, table := range schema.Tables {
			var tableName string
			if schema.Name == req.Catalog.DefaultSchema {
				tableName = table.Rel.Name
			} else {
				tableName = schema.Name + "_" + table.Rel.Name
			}
			structName := tableName
			if !options.EmitExactTableNames {
				structName = inflection.Singular(
					inflection.SingularParams{
						Name:       structName,
						Exclusions: options.InflectionExcludeTableNames,
					},
				)
			}
			s := Struct{
				Table:   &plugin.Identifier{Schema: schema.Name, Name: table.Rel.Name},
				Name:    StructName(structName, options),
				Comment: table.Comment,
			}
			for _, column := range table.Columns {
				tags := map[string]string{}
				if options.EmitDbTags {
					tags["db"] = column.Name
				}
				if options.EmitJsonTags {
					tags["json"] = JSONTagName(column.Name, options)
				}
				addExtraGoStructTags(tags, req, options, column)
				s.Fields = append(
					s.Fields, Field{
						Name:    StructName(column.Name, options),
						Type:    goType(req, options, column),
						Tags:    tags,
						Comment: column.Comment,
					},
				)
			}
			structs = append(structs, s)
		}
	}
	if len(structs) > 0 {
		sort.Slice(structs, func(i, j int) bool { return structs[i].Name < structs[j].Name })
	}
	return structs
}

type goColumn struct {
	id int
	*plugin.Column
	embed *goEmbed
}

type goEmbed struct {
	modelType string
	modelName string
	fields    []Field
}

// look through all the structs and attempt to find a matching one to embed
// We need the name of the struct and its field names.
func newGoEmbed(embed *plugin.Identifier, structs []Struct, defaultSchema string) *goEmbed {
	if embed == nil {
		return nil
	}

	for _, s := range structs {
		embedSchema := defaultSchema
		if embed.Schema != "" {
			embedSchema = embed.Schema
		}

		// compare the other attributes
		if embed.Catalog != s.Table.Catalog || embed.Name != s.Table.Name || embedSchema != s.Table.Schema {
			continue
		}

		fields := make([]Field, len(s.Fields))
		for i, f := range s.Fields {
			fields[i] = f
		}

		return &goEmbed{
			modelType: s.Name,
			modelName: s.Name,
			fields:    fields,
		}
	}

	return nil
}

func columnName(c *plugin.Column, pos int) string {
	if c.Name != "" {
		return c.Name
	}
	return fmt.Sprintf("column_%d", pos+1)
}

func paramName(p *plugin.Parameter) string {
	if p.Column.Name != "" {
		return argName(p.Column.Name)
	}
	return fmt.Sprintf("dollar_%d", p.Number)
}

func argName(name string) string {
	out := ""
	for i, p := range strings.Split(name, "_") {
		if i == 0 {
			out += strings.ToLower(p)
		} else if p == "id" {
			out += "ID"
		} else {
			out += strings.Title(p)
		}
	}
	return out
}

func buildQueries(req *plugin.GenerateRequest, options *opts.Options, structs []Struct) ([]Query, error) {
	qs := make([]Query, 0, len(req.Queries))
	for _, query := range req.Queries {
		if query.Name == "" {
			continue
		}
		if query.Cmd == "" {
			continue
		}

		var constantName string
		if options.EmitExportedQueries {
			constantName = sdk.Title(query.Name)
		} else {
			constantName = sdk.LowerTitle(query.Name)
		}

		paginated, cursorPagination, paginationComment, err := getPaginationFlags(query)
		if err != nil {
			return nil, err
		}

		comments := removeServiceComments(query.Comments)
		if options.EmitSqlAsComment {
			if len(comments) == 0 {
				comments = append(comments, query.Name)
			}
			comments = append(comments, " ")
			scanner := bufio.NewScanner(strings.NewReader(query.Text))
			for scanner.Scan() {
				line := scanner.Text()
				comments = append(comments, "  "+line)
			}
			if err := scanner.Err(); err != nil {
				return nil, err
			}
		}

		gq := Query{
			Cmd:              query.Cmd,
			ConstantName:     constantName,
			FieldName:        sdk.LowerTitle(query.Name) + "Stmt",
			MethodName:       query.Name,
			SourceName:       query.Filename,
			SQL:              query.Text,
			Comments:         comments,
			Table:            query.InsertIntoTable,
			Paginated:        paginated,
			CursorPagination: cursorPagination,
		}
		sqlpkg := parseDriver(options.SqlPackage)

		qpl := int(*options.QueryParameterLimit)

		if paginated {
			query.Params = append(query.Params, getPaginationParams(cursorPagination, len(query.Params))...)
		}

		if len(query.Params) == 1 && qpl != 0 {
			p := query.Params[0]
			gq.Arg = QueryValue{
				Name:      escape(paramName(p)),
				DBName:    p.Column.GetName(),
				Typ:       goType(req, options, p.Column),
				SQLDriver: sqlpkg,
				Column:    p.Column,
			}
		} else if len(query.Params) >= 1 {
			var cols []goColumn
			for _, p := range query.Params {
				cols = append(
					cols, goColumn{
						id:     int(p.Number),
						Column: p.Column,
					},
				)
			}
			s, err := columnsToStruct(req, options, gq.MethodName+"Params", cols, false)
			if err != nil {
				return nil, err
			}
			gq.Arg = QueryValue{
				Emit:        true,
				Name:        "arg",
				Struct:      s,
				SQLDriver:   sqlpkg,
				EmitPointer: options.EmitParamsStructPointers,
			}

			// if query params is 2, and query params limit is 4 AND this is a copyfrom, we still want to emit the query's model
			// otherwise we end up with a copyfrom using a struct without the struct definition
			if len(query.Params) <= qpl && query.Cmd != ":copyfrom" {
				gq.Arg.Emit = false
			}
		}

		if len(query.Columns) == 1 && query.Columns[0].EmbedTable == nil {
			c := query.Columns[0]
			name := columnName(c, 0)
			name = strings.Replace(name, "$", "_", -1)
			gq.Ret = QueryValue{
				Name:      escape(name),
				DBName:    name,
				Typ:       goType(req, options, c),
				SQLDriver: sqlpkg,
			}
		} else if putOutColumns(query) {
			var gs *Struct
			var emit bool

			for _, s := range structs {
				if len(s.Fields) != len(query.Columns) {
					continue
				}
				same := true
				fieldsWithColumns := make([]Field, len(query.Columns))
				for i, f := range s.Fields {
					c := query.Columns[i]
					sameName := f.Name == StructName(columnName(c, i), options)
					sameType := f.Type == goType(req, options, c)
					sameTable := sdk.SameTableName(c.Table, s.Table, req.Catalog.DefaultSchema)
					if !sameName || !sameType || !sameTable {
						same = false
						break
					}
					f.Column = c
					f.DBName = c.GetName()
					fieldsWithColumns[i] = f
				}
				if same {
					s.Fields = fieldsWithColumns
					gs = &s
					break
				}
			}

			if gs == nil {
				var columns []goColumn
				for i, c := range query.Columns {
					columns = append(
						columns, goColumn{
							id:     i,
							Column: c,
							embed:  newGoEmbed(c.EmbedTable, structs, req.Catalog.DefaultSchema),
						},
					)
				}
				var err error
				gs, err = columnsToStruct(req, options, gq.MethodName+"Row", columns, true)
				if err != nil {
					return nil, err
				}
				emit = true
			}
			gq.Ret = QueryValue{
				Emit:        emit,
				Name:        "i",
				Struct:      gs,
				SQLDriver:   sqlpkg,
				EmitPointer: options.EmitResultStructPointers,
			}
		}

		// pagination
		if gq.Paginated {
			if !gq.CursorPagination {
				gq.SQLPaginated = getOffsetPaginationSql(gq)
			} else {
				cursorFields, err := parseCursorFields(paginationComment, gq)
				if err != nil {
					return nil, err
				}
				gq.CursorFields = cursorFields
				gq.SQLPaginated = getCursorPaginationSql(gq, cursorFields)
			}
		}
		qs = append(qs, gq)
	}
	sort.Slice(qs, func(i, j int) bool { return qs[i].MethodName < qs[j].MethodName })

	return qs, nil
}

var cmdReturnsData = map[string]struct{}{
	metadata.CmdBatchMany: {},
	metadata.CmdBatchOne:  {},
	metadata.CmdMany:      {},
	metadata.CmdOne:       {},
}

func putOutColumns(query *plugin.Query) bool {
	_, found := cmdReturnsData[query.Cmd]
	return found
}

// It's possible that this method will generate duplicate JSON tag values
//
//	Columns: count, count,   count_2
//	 Fields: Count, Count_2, Count2
//
// JSON tags: count, count_2, count_2
//
// This is unlikely to happen, so don't fix it yet
func columnsToStruct(
	req *plugin.GenerateRequest,
	options *opts.Options,
	name string,
	columns []goColumn,
	useID bool,
) (*Struct, error) {
	gs := Struct{
		Name: name,
	}
	seen := map[string][]int{}
	suffixes := map[int]int{}
	for i, c := range columns {
		colName := columnName(c.Column, i)
		tagName := colName

		// override col/tag with expected model name
		if c.embed != nil {
			colName = c.embed.modelName
			tagName = SetCaseStyle(colName, "snake")
		}

		fieldName := StructName(colName, options)
		baseFieldName := fieldName
		// Track suffixes by the ID of the column, so that columns referring to the same numbered parameter can be
		// reused.
		suffix := 0
		if o, ok := suffixes[c.id]; ok && useID {
			suffix = o
		} else if v := len(seen[fieldName]); v > 0 && !c.IsNamedParam {
			suffix = v + 1
		}
		suffixes[c.id] = suffix
		if suffix > 0 {
			tagName = fmt.Sprintf("%s_%d", tagName, suffix)
			fieldName = fmt.Sprintf("%s_%d", fieldName, suffix)
		}
		tags := map[string]string{}
		if options.EmitDbTags {
			tags["db"] = tagName
		}
		if options.EmitJsonTags {
			tags["json"] = JSONTagName(tagName, options)
		}
		addExtraGoStructTags(tags, req, options, c.Column)
		f := Field{
			Name:   fieldName,
			DBName: colName,
			Tags:   tags,
			Column: c.Column,
		}
		if c.embed == nil {
			f.Type = goType(req, options, c.Column)
		} else {
			f.Type = c.embed.modelType
			f.EmbedFields = c.embed.fields
		}

		gs.Fields = append(gs.Fields, f)
		if _, found := seen[baseFieldName]; !found {
			seen[baseFieldName] = []int{i}
		} else {
			seen[baseFieldName] = append(seen[baseFieldName], i)
		}
	}

	// If a field does not have a known type, but another
	// field with the same name has a known type, assign
	// the known type to the field without a known type
	for i, field := range gs.Fields {
		if len(seen[field.Name]) > 1 && field.Type == "interface{}" {
			for _, j := range seen[field.Name] {
				if i == j {
					continue
				}
				otherField := gs.Fields[j]
				if otherField.Type != field.Type {
					field.Type = otherField.Type
				}
				gs.Fields[i] = field
			}
		}
	}

	err := checkIncompatibleFieldTypes(gs.Fields)
	if err != nil {
		return nil, err
	}

	return &gs, nil
}

func checkIncompatibleFieldTypes(fields []Field) error {
	fieldTypes := map[string]string{}
	for _, field := range fields {
		if fieldType, found := fieldTypes[field.Name]; !found {
			fieldTypes[field.Name] = field.Type
		} else if field.Type != fieldType {
			return fmt.Errorf("named param %s has incompatible types: %s, %s", field.Name, field.Type, fieldType)
		}
	}
	return nil
}

func addPageTypesToStructs(structs []Struct, queries []Query) []Struct {
	for _, q := range queries {
		if q.Paginated && q.Ret.Struct != nil {
			if q.CursorPagination {
				structs = addConnectionStruct(*q.Ret.Struct, structs, q.CursorFields)
			} else {
				structs = addPageStruct(*q.Ret.Struct, structs)
			}
		}
	}
	return structs
}

func addPageStruct(original Struct, structs []Struct) []Struct {
	pageName := original.Name + "Page"
	for _, s := range structs {
		if s.Name == pageName {
			return structs
		}
	}
	pageStruct := Struct{
		Name: pageName,
		Fields: []Field{
			{
				Name:    "Items",
				DBName:  "",
				Type:    "[]" + original.Name,
				Comment: "",
				Column: &plugin.Column{
					Name:    "items",
					NotNull: true,
					IsArray: true,
					Type:    &plugin.Identifier{Name: original.Name},
				},
				EmbedFields: nil,
			},
			{
				Name:   "Total",
				DBName: "",
				Type:   "int",
			},
			{
				Name:   "HasNext",
				DBName: "",
				Type:   "bool",
			},
		},
	}
	structs = append(structs, pageStruct)
	return structs
}

func addConnectionStruct(original Struct, structs []Struct, cursorFields []CursorField) []Struct {
	connectionName := original.Name + "Connection"
	edgeName := original.Name + "Edge"
	hasPageInfo := false
	for _, s := range structs {
		if s.Name == connectionName {
			return structs
		}
		if s.Name == "PageInfo" {
			hasPageInfo = true
		}
	}

	edgeStruct := Struct{
		Name: edgeName,
		Fields: []Field{
			{
				Name: "Node",
				Type: original.Name,
				Column: &plugin.Column{
					Name:    "node",
					NotNull: true,
					IsArray: false,
					Type:    &plugin.Identifier{Name: original.Name},
				},
			},
			{
				Name: "Cursor",
				Type: "string",
				Column: &plugin.Column{
					Name:    "cursor",
					NotNull: true,
					IsArray: false,
					Type:    &plugin.Identifier{Name: "string"},
				},
			},
		},
	}

	connectionStruct := Struct{
		Name: connectionName,
		Fields: []Field{
			{
				Name:    "Edges",
				DBName:  "",
				Type:    "[]" + edgeName,
				Comment: "",
				Column: &plugin.Column{
					Name:    "edges",
					NotNull: true,
					IsArray: true,
					Type:    &plugin.Identifier{Name: edgeName},
				},
				EmbedFields: nil,
			},
			{
				Name:   "PageInfo",
				DBName: "",
				Type:   "PageInfo",
				Column: &plugin.Column{
					Name:    "pageInfo",
					NotNull: true,
					IsArray: false,
					Type:    &plugin.Identifier{Name: "PageInfo"},
				},
			},
		},
	}

	if !hasPageInfo {
		pageInfoStruct := Struct{
			Name: "PageInfo",
			Fields: []Field{
				{
					Name:   "StartCursor",
					DBName: "",
					Type:   "string",
				},
				{
					Name:   "EndCursor",
					DBName: "",
					Type:   "string",
				},
				{
					Name:   "HasNextPage",
					DBName: "",
					Type:   "bool",
				},
				{
					Name:   "HasPreviousPage",
					DBName: "",
					Type:   "bool",
				},
			},
		}
		structs = append(structs, pageInfoStruct)
	}

	cursorStruct := Struct{
		Name:   sdk.LowerTitle(original.Name) + "Cursor",
		Fields: make([]Field, 0, len(cursorFields)),
	}

	for _, cursorField := range cursorFields {
		cursorStruct.Fields = append(cursorStruct.Fields, cursorField.Field)
	}

	structs = append(structs, connectionStruct, edgeStruct, cursorStruct)
	return structs
}

func parseCursorFields(comment string, query Query) ([]CursorField, error) {
	paramsParts := strings.Split(comment, "cursor:")
	if len(paramsParts) != 2 {
		return nil, fmt.Errorf(
			"%s: invalid cursor comment (%s): it should have 'cursor:' substring (e.g. -- paginated: cursor:-created_at,id)",
			query.MethodName,
			comment,
		)
	}
	comment = paramsParts[1]
	parts := strings.Split(comment, ",")
	params := make([]CursorField, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		isAsc := true
		if strings.HasPrefix(part, "-") {
			isAsc = false
			part = part[1:]
		}
		if query.Ret.Struct == nil {
			return nil, fmt.Errorf("%s: cursor pagination requires a struct return type", query.MethodName)
		}
		for _, field := range query.Ret.Struct.Fields {
			if field.DBName == part {
				params = append(
					params, CursorField{
						Field: field,
						IsAsc: isAsc,
					},
				)
				break
			}
		}
	}
	if len(params) != len(parts) {
		return nil, fmt.Errorf(
			"%s: cursor pagination requires all fields from cursor to be present in the result struct",
			query.MethodName,
		)
	}
	return params, nil
}

func getPaginationParams(isCursorPagination bool, paramsCount int) []*plugin.Parameter {
	params := make([]*plugin.Parameter, 0)
	number := int32(paramsCount + 1)
	if isCursorPagination {
		params = append(
			params, &plugin.Parameter{
				Number: number,
				Column: &plugin.Column{
					Name:         "limit",
					NotNull:      true,
					IsNamedParam: true,
					Type: &plugin.Identifier{
						Name: "int",
					},
				},
			}, &plugin.Parameter{
				Number: number + 1,
				Column: &plugin.Column{
					Name:         "cursor",
					NotNull:      true,
					IsNamedParam: true,
					Type: &plugin.Identifier{
						Name: "string",
					},
				},
			},
		)

		return params
	}
	params = append(
		params, &plugin.Parameter{
			Number: number,
			Column: &plugin.Column{
				Name:         "limit",
				NotNull:      true,
				IsNamedParam: true,
				Type: &plugin.Identifier{
					Name: "int",
				},
			},
		}, &plugin.Parameter{
			Number: number + 1,
			Column: &plugin.Column{
				Name:         "offset",
				NotNull:      true,
				IsNamedParam: true,
				Type: &plugin.Identifier{
					Name: "int",
				},
			},
		},
	)

	return params
}

func getPaginationFlags(query *plugin.Query) (paginated, cursorPagination bool, paginationComment string, err error) {
	comments := query.Comments
	for _, comment := range comments {
		comment = strings.TrimSpace(comment)
		if strings.HasPrefix(comment, "paginated") {
			paginated = true
			paginationComment = comment
			if strings.Contains(comment, "cursor") {
				cursorPagination = true
			}
			break
		}
	}
	if paginated && query.Cmd != metadata.CmdMany {
		err = fmt.Errorf("%s: query is marked as paginated but is not a :many query", query.Name)
		return
	}
	if paginated && queryHasLimitRegexp.MatchString(query.Text) {
		err = fmt.Errorf("%s: using LIMIT in paginated query is forbidden", query.Name)
		return
	}
	if paginated && queryHasOffsetRegexp.MatchString(query.Text) {
		err = fmt.Errorf("%s: using OFFSET in paginated query is forbidden", query.Name)
		return
	}
	if paginated && cursorPagination && len(query.Columns) < 2 {
		err = fmt.Errorf("%s: cursor pagination requires a query with at least two columns", query.Name)
		return
	}
	if paginated && cursorPagination && regexpOrderBy.MatchString(query.Text) {
		err = fmt.Errorf("%s: cursor pagination requires a query without ORDER BY clause", query.Name)
		return
	}
	return
}

func removeServiceComments(comments []string) []string {
	var newComments []string
	for _, comment := range comments {
		if strings.HasPrefix(comment, "gql:") {
			continue
		}
		if strings.HasPrefix(comment, "paginated:") {
			continue
		}
		if strings.HasPrefix(comment, "gql-comment:") {
			continue
		}
		if strings.HasPrefix(comment, "gql-end") {
			continue
		}
		newComments = append(newComments, comment)
	}
	return newComments
}

func getOffsetPaginationSql(query Query) string {
	sql := query.SQL
	limit := 0
	offset := 0
	for i, f := range query.Arg.Struct.Fields {
		if f.Name == "limit" {
			limit = i + 1
		}
		if f.Name == "offset" {
			offset = i + 1
		}
	}
	return fmt.Sprintf("%s LIMIT %d OFFSET %d", sql, limit, offset)
}

func getCursorPaginationSql(query Query, cursorFields []CursorField) string {
	sql := query.SQL
	limit := 0
	cursor := 0
	for i, f := range query.Arg.Struct.Fields {
		if f.DBName == "limit" {
			limit = i + 1
		}
		if f.DBName == "cursor" {
			cursor = i + 1
		}
	}
	cursorWhereClause := ""
	orderClause := ""
	for i, f := range cursorFields {
		if i > 0 {
			cursorWhereClause += " AND"
			orderClause += ", "
		}
		orderClause += f.Field.Column.GetName()
		sign := ">"
		if !f.IsAsc {
			sign = "<"
			orderClause += " DESC"
		}
		cursorWhereClause += fmt.Sprintf(" (%s %s $%d", f.Field.Column.GetName(), sign, cursor+i+1)
		if i < len(cursorFields)-1 {
			cursorWhereClause += fmt.Sprintf(" OR (%s = $%d", f.Field.Column.GetName(), cursor+i+1)
		}
	}
	for i := 0; i < len(cursorFields)+1; i++ {
		cursorWhereClause += ")"
	}
	sql = fmt.Sprintf(
		`SELECT cursor_pagination_source.* 
FROM (%s) as cursor_pagination_source
WHERE $%d='' or %s
ORDER BY %s
LIMIT $%d`, sql, cursor, cursorWhereClause, orderClause, limit,
	)

	return sql
}
