package metrics

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	_ "github.com/lib/pq"
)

type Querier struct {
	db     *sql.DB
	dbname string
}

type TableInfo struct {
	Name       string
	Columns    []string
	JSONFields []string // JSONB fields inside 'data' column
}

type QueryResult struct {
	Columns []string
	Rows    [][]string
}

func NewQuerier(connStr, dbname string) (*Querier, error) {
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open connection: %w", err)
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to connect to pgwatch_metrics DB: %w", err)
	}
	return &Querier{db: db, dbname: dbname}, nil
}

func (q *Querier) Dbname() string {
	return q.dbname
}

func (q *Querier) SchemaContext() (string, error) {
	tables, err := q.listTables()
	if err != nil {
		return "", err
	}

	var sb strings.Builder
	sb.WriteString("pgwatch metrics database schema:\n\n")
	for _, t := range tables {
		sb.WriteString(fmt.Sprintf("Table: %s\n  Columns: %s\n", t.Name, strings.Join(t.Columns, ", ")))
		if len(t.JSONFields) > 0 {
			sb.WriteString(fmt.Sprintf("  JSONB 'data' fields: %s\n", strings.Join(t.JSONFields, ", ")))
		}
		sb.WriteString("\n")
	}
	sb.WriteString(fmt.Sprintf("IMPORTANT:\n"))
	sb.WriteString(fmt.Sprintf("- Always scope queries with: WHERE dbname = '%s'\n", q.dbname))
	sb.WriteString("- Metrics are stored as JSONB in the 'data' column, access fields like: data->>'field_name'\n")
	sb.WriteString("- The 'sys_id' field inside data JSONB uniquely identifies the pgwatch instance\n")
	return sb.String(), nil
}

func (q *Querier) RunQuery(query string) (*QueryResult, error) {
	if !strings.Contains(strings.ToLower(query), "dbname") {
		return nil, fmt.Errorf("query must be scoped with dbname for safety")
	}

	rows, err := q.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	result := &QueryResult{Columns: cols}
	for rows.Next() {
		vals := make([]interface{}, len(cols))
		ptrs := make([]interface{}, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		rows.Scan(ptrs...)

		row := make([]string, len(cols))
		for i, v := range vals {
			if v == nil {
				row[i] = "NULL"
			} else {
				switch val := v.(type) {
				case []byte:
					row[i] = string(val)
				default:
					row[i] = fmt.Sprintf("%v", val)
				}
			}
		}
		result.Rows = append(result.Rows, row)
	}
	return result, nil
}

func (q *Querier) listTables() ([]TableInfo, error) {
	rows, err := q.db.Query(`
		SELECT table_name 
		FROM information_schema.tables 
		WHERE table_schema = 'public' 
		  AND table_type = 'BASE TABLE'
		ORDER BY table_name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tables []TableInfo
	for rows.Next() {
		var name string
		rows.Scan(&name)
		cols, _ := q.getColumns(name)
		jsonFields, _ := q.getJSONBFields(name)
		tables = append(tables, TableInfo{
			Name:       name,
			Columns:    cols,
			JSONFields: jsonFields,
		})
	}
	return tables, nil
}

func (q *Querier) getColumns(table string) ([]string, error) {
	rows, err := q.db.Query(`
		SELECT column_name 
		FROM information_schema.columns 
		WHERE table_schema = 'public' AND table_name = $1
		ORDER BY ordinal_position
	`, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cols []string
	for rows.Next() {
		var col string
		rows.Scan(&col)
		cols = append(cols, col)
	}
	return cols, nil
}

// getJSONBFields samples one row to discover keys inside the 'data' JSONB column
func (q *Querier) getJSONBFields(table string) ([]string, error) {
	var data []byte
	err := q.db.QueryRow(fmt.Sprintf(
		`SELECT data FROM %s WHERE dbname = $1 AND data IS NOT NULL LIMIT 1`, table,
	), q.dbname).Scan(&data)
	if err != nil || data == nil {
		return nil, nil
	}

	var obj map[string]interface{}
	if err := json.Unmarshal(data, &obj); err != nil {
		return nil, nil
	}

	var fields []string
	for k := range obj {
		fields = append(fields, k)
	}
	return fields, nil
}

func (q *Querier) Close() {
	q.db.Close()
}
