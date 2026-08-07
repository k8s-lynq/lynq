/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package datasource

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"time"

	_ "github.com/go-sql-driver/mysql" // MySQL driver
)

// Connection pool defaults, applied when Config leaves the corresponding field unset.
//
// Sized to what a hub actually uses — one connection — plus a little headroom. See
// NewMySQLAdapter for why anything larger is only a ceiling, never an allocation.
const (
	defaultMaxOpenConns    = 3
	defaultMaxIdleConns    = 2
	defaultConnMaxLifetime = 5 * time.Minute
)

// poolSettings holds the effective connection-pool configuration for one adapter.
type poolSettings struct {
	maxOpenConns    int
	maxIdleConns    int
	connMaxLifetime time.Duration
}

// resolvePoolSettings fills in defaults for anything Config leaves unset.
//
// The defaults are sized to what a hub actually uses: exactly one connection. QueryNodes runs
// a single QueryContext per sync and returns the connection via `defer rows.Close()`, and
// controller-runtime processes at most one reconcile per hub at a time, so there is never a
// second concurrent query. database/sql opens connections lazily, so a larger ceiling was never
// an allocation — it just failed to describe what the code does. These values state the real
// requirement and cap what a future concurrent caller could open against the user's database,
// while leaving one connection of headroom for the overlap when ConnMaxLifetime recycles.
//
// An unparseable ConnMaxLifetime falls back to the default rather than failing: connection
// recycling is a tuning detail, not something worth refusing to sync over.
func resolvePoolSettings(config Config) poolSettings {
	settings := poolSettings{
		maxOpenConns:    config.MaxOpenConns,
		maxIdleConns:    config.MaxIdleConns,
		connMaxLifetime: defaultConnMaxLifetime,
	}
	if settings.maxOpenConns == 0 {
		settings.maxOpenConns = defaultMaxOpenConns
	}
	if settings.maxIdleConns == 0 {
		settings.maxIdleConns = defaultMaxIdleConns
	}
	if config.ConnMaxLifetime != "" {
		if parsed, err := time.ParseDuration(config.ConnMaxLifetime); err == nil {
			settings.connMaxLifetime = parsed
		}
	}
	return settings
}

// MySQLAdapter implements the Datasource interface for MySQL
type MySQLAdapter struct {
	db *sql.DB
}

// NewMySQLAdapter creates a new MySQL datasource adapter
func NewMySQLAdapter(config Config) (*MySQLAdapter, error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=true",
		config.Username,
		config.Password,
		config.Host,
		config.Port,
		config.Database,
	)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open MySQL connection: %w", err)
	}

	settings := resolvePoolSettings(config)
	db.SetMaxOpenConns(settings.maxOpenConns)
	db.SetMaxIdleConns(settings.maxIdleConns)
	db.SetConnMaxLifetime(settings.connMaxLifetime)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close() // Best effort close on error
		return nil, fmt.Errorf("failed to ping MySQL: %w", err)
	}

	return &MySQLAdapter{db: db}, nil
}

// QueryNodes queries active nodes from the MySQL database
func (a *MySQLAdapter) QueryNodes(ctx context.Context, config QueryConfig) ([]NodeRow, error) {
	// Build column list - start with required fields
	columns := []string{
		config.ValueMappings.UID,
	}

	// Add HostOrURL only if specified (deprecated, optional since v1.1.11)
	includeHostOrURL := config.ValueMappings.HostOrURL != ""
	if includeHostOrURL {
		columns = append(columns, config.ValueMappings.HostOrURL)
	}

	// Add activate column
	columns = append(columns, config.ValueMappings.Activate)

	// Add extra columns in sorted order for stable queries
	// Sort the keys to ensure consistent column order
	extraKeys := make([]string, 0, len(config.ExtraMappings))
	for key := range config.ExtraMappings {
		extraKeys = append(extraKeys, key)
	}
	sort.Strings(extraKeys)

	extraColumns := make([]string, 0, len(config.ExtraMappings))
	for _, key := range extraKeys {
		col := config.ExtraMappings[key]
		columns = append(columns, col)
		extraColumns = append(extraColumns, col)
	}

	// Build query
	query := fmt.Sprintf("SELECT %s FROM %s", joinColumns(columns), config.Table)

	// Execute query
	rows, err := a.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query nodes: %w", err)
	}
	defer func() {
		_ = rows.Close() // Best effort close
	}()

	// Scan results
	var nodes []NodeRow
	for rows.Next() {
		row := NodeRow{
			Extra: make(map[string]string),
		}

		// Use NullString for required fields to handle NULL values
		var uid, hostOrURL, activate sql.NullString

		// Prepare scan destinations based on which columns were queried
		scanDest := []interface{}{&uid}
		if includeHostOrURL {
			scanDest = append(scanDest, &hostOrURL)
		}
		scanDest = append(scanDest, &activate)

		// Add extra column destinations
		extraValues := make([]sql.NullString, len(extraColumns))
		for i := range extraValues {
			scanDest = append(scanDest, &extraValues[i])
		}

		if err := rows.Scan(scanDest...); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		// Convert NullString to string (NULL becomes empty string)
		if uid.Valid {
			row.UID = uid.String
		}
		if hostOrURL.Valid {
			row.HostOrURL = hostOrURL.String
		}
		if activate.Valid {
			row.Activate = activate.String
		}

		// Map extra values - Build column index map first for stable mapping
		colIndex := make(map[string]int)
		for i, col := range extraColumns {
			colIndex[col] = i
		}

		// Map extra values using stable indices
		for key, col := range config.ExtraMappings {
			idx, ok := colIndex[col]
			if !ok {
				// Column not in result set (shouldn't happen)
				row.Extra[key] = ""
				continue
			}
			if extraValues[idx].Valid {
				row.Extra[key] = extraValues[idx].String
			} else {
				row.Extra[key] = "" // Null values become empty strings
			}
		}

		// Filter: only include active nodes
		// Note: HostOrURL is deprecated since v1.1.11 and no longer required
		if isActive(row.Activate) {
			nodes = append(nodes, row)
		}
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return nodes, nil
}

// Close closes the database connection
func (a *MySQLAdapter) Close() error {
	if a.db != nil {
		return a.db.Close()
	}
	return nil
}

// Helper functions

func joinColumns(columns []string) string {
	result := ""
	for i, col := range columns {
		if i > 0 {
			result += ", "
		}
		result += "`" + col + "`" // Escape column names
	}
	return result
}

func isActive(value string) bool {
	// Truthy values: "1", "true", "TRUE", "yes", etc.
	switch value {
	case "1", "true", "TRUE", "True", "yes", "YES", "Yes":
		return true
	default:
		return false
	}
}
