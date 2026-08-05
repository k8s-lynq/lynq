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
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewMySQLAdapter(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			name: "valid configuration with defaults",
			config: Config{
				Host:     "localhost",
				Port:     3306,
				Username: "root",
				Password: "password",
				Database: "nodes",
			},
			wantErr: true, // Will fail without real MySQL, but tests config parsing
		},
		{
			name: "valid configuration with custom pool settings",
			config: Config{
				Host:            "localhost",
				Port:            3306,
				Username:        "root",
				Password:        "password",
				Database:        "nodes",
				MaxOpenConns:    50,
				MaxIdleConns:    10,
				ConnMaxLifetime: "10m",
			},
			wantErr: true, // Will fail without real MySQL
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewMySQLAdapter(tt.config)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestMySQLAdapter_QueryNodes(t *testing.T) {
	tests := []struct {
		name          string
		queryConfig   QueryConfig
		setupMock     func(sqlmock.Sqlmock)
		want          []NodeRow
		wantErr       bool
		errorContains string
	}{
		{
			name: "successful query with active nodes",
			queryConfig: QueryConfig{
				Table: "nodes",
				ValueMappings: ValueMappings{
					UID:       "id",
					HostOrURL: "url",
					Activate:  "active",
				},
				ExtraMappings: map[string]string{
					"planId": "subscription_plan",
					"region": "deployment_region",
				},
			},
			setupMock: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"id", "url", "active", "subscription_plan", "deployment_region"}).
					AddRow("node1", "https://node1.example.com", "1", "premium", "us-east-1").
					AddRow("node2", "https://node2.example.com", "true", "basic", "us-west-2").
					AddRow("node3", "https://node3.example.com", "0", "premium", "eu-west-1"). // Inactive - should be filtered
					AddRow("node4", "", "1", "basic", "ap-south-1")                            // Empty URL - now allowed (v1.1.11+)
				mock.ExpectQuery("SELECT .* FROM .*").
					WillReturnRows(rows)
			},
			want: []NodeRow{
				{
					UID:       "node1",
					HostOrURL: "https://node1.example.com",
					Activate:  "1",
					Extra: map[string]string{
						"planId": "premium",
						"region": "us-east-1",
					},
				},
				{
					UID:       "node2",
					HostOrURL: "https://node2.example.com",
					Activate:  "true",
					Extra: map[string]string{
						"planId": "basic",
						"region": "us-west-2",
					},
				},
				{
					UID:       "node4",
					HostOrURL: "", // Empty URL now allowed since v1.1.11
					Activate:  "1",
					Extra: map[string]string{
						"planId": "basic",
						"region": "ap-south-1",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "query with NULL values",
			queryConfig: QueryConfig{
				Table: "nodes",
				ValueMappings: ValueMappings{
					UID:       "id",
					HostOrURL: "url",
					Activate:  "active",
				},
				ExtraMappings: map[string]string{
					"config": "json_config",
				},
			},
			setupMock: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"id", "url", "active", "json_config"}).
					AddRow("node1", "https://node1.example.com", "1", sql.NullString{Valid: false}). // NULL config
					AddRow(sql.NullString{Valid: false}, "https://node2.example.com", "1", "{}").    // NULL id
					AddRow("node3", sql.NullString{Valid: false}, "1", "{}").                        // NULL URL - now allowed (v1.1.11+)
					AddRow("node4", "https://node4.example.com", sql.NullString{Valid: false}, "{}") // NULL activate - filtered out
				mock.ExpectQuery("SELECT .* FROM .*").
					WillReturnRows(rows)
			},
			want: []NodeRow{
				{
					UID:       "node1",
					HostOrURL: "https://node1.example.com",
					Activate:  "1",
					Extra: map[string]string{
						"config": "",
					},
				},
				{
					UID:       "", // NULL UID is converted to empty string but still included
					HostOrURL: "https://node2.example.com",
					Activate:  "1",
					Extra: map[string]string{
						"config": "{}",
					},
				},
				{
					UID:       "node3",
					HostOrURL: "", // NULL URL now allowed since v1.1.11
					Activate:  "1",
					Extra: map[string]string{
						"config": "{}",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "query with no extra mappings",
			queryConfig: QueryConfig{
				Table: "nodes",
				ValueMappings: ValueMappings{
					UID:       "node_id",
					HostOrURL: "node_url",
					Activate:  "is_active",
				},
				ExtraMappings: nil,
			},
			setupMock: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"node_id", "node_url", "is_active"}).
					AddRow("t1", "https://t1.example.com", "yes").
					AddRow("t2", "https://t2.example.com", "YES")
				mock.ExpectQuery("SELECT .* FROM .*").
					WillReturnRows(rows)
			},
			want: []NodeRow{
				{
					UID:       "t1",
					HostOrURL: "https://t1.example.com",
					Activate:  "yes",
					Extra:     map[string]string{},
				},
				{
					UID:       "t2",
					HostOrURL: "https://t2.example.com",
					Activate:  "YES",
					Extra:     map[string]string{},
				},
			},
			wantErr: false,
		},
		{
			name: "query without hostOrUrl mapping (v1.1.11+ - deprecated field is optional)",
			queryConfig: QueryConfig{
				Table: "nodes",
				ValueMappings: ValueMappings{
					UID:       "node_id",
					HostOrURL: "", // No hostOrUrl mapping (deprecated since v1.1.11)
					Activate:  "is_active",
				},
				ExtraMappings: map[string]string{
					"customUrl": "custom_url", // Use extraValueMappings instead
					"planId":    "plan_id",
				},
			},
			setupMock: func(mock sqlmock.Sqlmock) {
				// Only query UID, activate, and extra columns (no hostOrUrl)
				rows := sqlmock.NewRows([]string{"node_id", "is_active", "custom_url", "plan_id"}).
					AddRow("node1", "1", "https://custom1.example.com", "premium").
					AddRow("node2", "true", "https://custom2.example.com", "basic").
					AddRow("node3", "0", "https://custom3.example.com", "free") // Inactive - filtered
				mock.ExpectQuery("SELECT .* FROM .*").
					WillReturnRows(rows)
			},
			want: []NodeRow{
				{
					UID:       "node1",
					HostOrURL: "", // Not queried, remains empty
					Activate:  "1",
					Extra: map[string]string{
						"customUrl": "https://custom1.example.com",
						"planId":    "premium",
					},
				},
				{
					UID:       "node2",
					HostOrURL: "", // Not queried, remains empty
					Activate:  "true",
					Extra: map[string]string{
						"customUrl": "https://custom2.example.com",
						"planId":    "basic",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "database query error",
			queryConfig: QueryConfig{
				Table: "nodes",
				ValueMappings: ValueMappings{
					UID:       "id",
					HostOrURL: "url",
					Activate:  "active",
				},
			},
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT .* FROM .*").
					WillReturnError(sql.ErrConnDone)
			},
			wantErr:       true,
			errorContains: "failed to query nodes",
		},
		{
			name: "scan error",
			queryConfig: QueryConfig{
				Table: "nodes",
				ValueMappings: ValueMappings{
					UID:       "id",
					HostOrURL: "url",
					Activate:  "active",
				},
			},
			setupMock: func(mock sqlmock.Sqlmock) {
				// Return fewer columns than expected (will cause scan error)
				rows := sqlmock.NewRows([]string{"id", "url"}).
					AddRow("node1", "https://node1.example.com")
				mock.ExpectQuery("SELECT .* FROM .*").
					WillReturnRows(rows)
			},
			wantErr:       true,
			errorContains: "failed to scan row",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock database
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer func() {
				_ = db.Close()
			}()

			// Setup mock expectations
			tt.setupMock(mock)

			// Create adapter with mocked database
			adapter := &MySQLAdapter{db: db}

			// Execute query
			ctx := context.Background()
			got, err := adapter.QueryNodes(ctx, tt.queryConfig)

			// Check error
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}

			// Ensure all expectations were met
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestJoinColumns(t *testing.T) {
	tests := []struct {
		name    string
		columns []string
		want    string
	}{
		{
			name:    "single column",
			columns: []string{"id"},
			want:    "`id`",
		},
		{
			name:    "multiple columns",
			columns: []string{"id", "name", "email"},
			want:    "`id`, `name`, `email`",
		},
		{
			name:    "empty columns",
			columns: []string{},
			want:    "",
		},
		{
			name:    "columns with special characters",
			columns: []string{"user.id", "node-name"},
			want:    "`user.id`, `node-name`",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := joinColumns(tt.columns)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestIsActive(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  bool
	}{
		// Truthy values
		{name: "numeric 1", value: "1", want: true},
		{name: "lowercase true", value: "true", want: true},
		{name: "uppercase TRUE", value: "TRUE", want: true},
		{name: "titlecase True", value: "True", want: true},
		{name: "lowercase yes", value: "yes", want: true},
		{name: "uppercase YES", value: "YES", want: true},
		{name: "titlecase Yes", value: "Yes", want: true},

		// Falsy values
		{name: "numeric 0", value: "0", want: false},
		{name: "lowercase false", value: "false", want: false},
		{name: "uppercase FALSE", value: "FALSE", want: false},
		{name: "empty string", value: "", want: false},
		{name: "no", value: "no", want: false},
		{name: "NO", value: "NO", want: false},
		{name: "random string", value: "random", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isActive(tt.value)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestMySQLAdapter_Close(t *testing.T) {
	// Create mock database
	db, mock, err := sqlmock.New()
	require.NoError(t, err)

	// Expect Close to be called
	mock.ExpectClose()

	adapter := &MySQLAdapter{db: db}

	// Close should not return error
	err = adapter.Close()
	assert.NoError(t, err)

	// Ensure all expectations were met
	assert.NoError(t, mock.ExpectationsWereMet())

	// Closing a nil adapter should not panic
	nilAdapter := &MySQLAdapter{db: nil}
	err = nilAdapter.Close()
	assert.NoError(t, err)
}

// TestResolvePoolSettings pins the connection-pool defaults.
//
// These are not arbitrary numbers: a hub issues exactly one query per sync and returns the
// connection immediately, and controller-runtime never reconciles the same hub concurrently,
// so one connection is all a hub uses. Now that the pool holds datasources open across syncs
// rather than discarding them, these values are what actually bounds the operator's footprint
// on the user's database — so a change to them should be a deliberate edit here, not a drift.
func TestResolvePoolSettings(t *testing.T) {
	tests := []struct {
		name   string
		config Config
		want   poolSettings
	}{
		{
			name:   "unset config uses defaults sized to a hub's single connection",
			config: Config{},
			want: poolSettings{
				maxOpenConns:    3,
				maxIdleConns:    2,
				connMaxLifetime: 5 * time.Minute,
			},
		},
		{
			name: "explicit settings win",
			config: Config{
				MaxOpenConns:    10,
				MaxIdleConns:    4,
				ConnMaxLifetime: "90s",
			},
			want: poolSettings{
				maxOpenConns:    10,
				maxIdleConns:    4,
				connMaxLifetime: 90 * time.Second,
			},
		},
		{
			name:   "unparseable lifetime falls back rather than failing the sync",
			config: Config{ConnMaxLifetime: "not-a-duration"},
			want: poolSettings{
				maxOpenConns:    3,
				maxIdleConns:    2,
				connMaxLifetime: 5 * time.Minute,
			},
		},
		{
			name:   "partial config only defaults what is missing",
			config: Config{MaxOpenConns: 7},
			want: poolSettings{
				maxOpenConns:    7,
				maxIdleConns:    2,
				connMaxLifetime: 5 * time.Minute,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, resolvePoolSettings(tt.config))
		})
	}

	// Idle must not exceed open: database/sql silently reduces maxIdle to maxOpen, which would
	// make the configured values quietly disagree with reality.
	defaults := resolvePoolSettings(Config{})
	assert.LessOrEqual(t, defaults.maxIdleConns, defaults.maxOpenConns,
		"maxIdleConns must not exceed maxOpenConns")
}
