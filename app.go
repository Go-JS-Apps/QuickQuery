package main

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "embed"

	_ "github.com/lib/pq"
	_ "github.com/microsoft/go-mssqldb"
	"github.com/tidwall/gjson"
)

//go:embed wails.json
var wailsJSON string

// App struct
type App struct {
	ctx context.Context
}

type QueryResult struct {
	Result  []map[string]interface{} `json:"result"`
	Columns []string                 `json:"columns"`
	Ms      int64                    `json:"ms"`
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

// Get app version from wails.json file
func (a *App) GetAppVersion() string {
	version := gjson.Get(wailsJSON, "info.productVersion")
	return version.String()
}

// Greet returns a greeting for the given name
func (a *App) Greet(name string) string {
	return fmt.Sprintf("Hello %s, It's show time!", name)
}

func (a *App) MssqlQuery(server string, user string, password string, database string, query string) (*QueryResult, error) {
	start := time.Now()

	db, err := getDBConnection(server, user, password, database)
	if err != nil {
		return nil, err
	}

	result, columns, err := execQuery(db, query)
	if err != nil {
		return nil, err
	}

	println("query executed!")

	return &QueryResult{
		Result:  result,
		Columns: columns,
		Ms:      time.Since(start).Milliseconds(),
	}, nil
}

// PostgresQuery executes a query against a PostgreSQL database
func (a *App) PostgresQuery(server string, user string, password string, database string, query string) (*QueryResult, error) {
	start := time.Now()

	// Format connection string for PostgreSQL
	connString := fmt.Sprintf("host=%s user=%s password=%s dbname=%s sslmode=disable",
		server, user, password, database)

	// Open connection
	db, err := sql.Open("postgres", connString)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	// Execute query
	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Get column names
	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	// Prepare result
	result := make([]map[string]interface{}, 0)

	// Scan rows
	for rows.Next() {
		// Create a slice of interface{} to hold the values
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))

		// Set up pointers to each interface{} value
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		// Scan the row into the slice of interface{}
		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, err
		}

		// Create a map for this row
		rowMap := make(map[string]interface{})

		// Convert any nil values to nil interface{} for JSON marshaling
		for i, col := range columns {
			val := values[i]

			// Handle nil values
			if val == nil {
				rowMap[col] = nil
				continue
			}

			// Handle byte slices (bytea in PostgreSQL)
			if b, ok := val.([]byte); ok {
				rowMap[col] = string(b)
			} else {
				rowMap[col] = val
			}
		}

		// Add the row to the result
		result = append(result, rowMap)
	}

	// Check for errors from iterating over rows
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Calculate execution time
	ms := time.Since(start).Milliseconds()

	return &QueryResult{
		Result:  result,
		Columns: columns,
		Ms:      ms,
	}, nil
}

// PostgresGetDatabases retrieves a list of PostgreSQL databases
func (a *App) PostgresGetDatabases(server string, user string, password string) ([]string, error) {
	// Format connection string for PostgreSQL - connect to 'postgres' database initially
	connString := fmt.Sprintf("host=%s user=%s password=%s dbname=postgres sslmode=disable",
		server, user, password)

	// Open connection
	db, err := sql.Open("postgres", connString)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	// Query to list all databases
	rows, err := db.Query("SELECT datname FROM pg_database WHERE datistemplate = false ORDER BY datname")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Collect database names
	var databases []string
	for rows.Next() {
		var dbName string
		if err := rows.Scan(&dbName); err != nil {
			return nil, err
		}
		databases = append(databases, dbName)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return databases, nil
}
