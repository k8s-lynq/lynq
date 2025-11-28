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

package e2e

import (
	"fmt"
	"os/exec"

	"github.com/k8s-lynq/lynq/test/utils"
)

// TestDatasourceAdapter defines the interface for datasource-specific test operations.
// Implement this interface to add support for new datasource types (e.g., PostgreSQL).
type TestDatasourceAdapter interface {
	// GetType returns the datasource type (e.g., "mysql", "postgresql")
	GetType() string

	// GetServiceHost returns the service host for the datasource
	GetServiceHost(namespace string) string

	// GetDefaultPort returns the default port for the datasource
	GetDefaultPort() int

	// CreateTable creates a table with the given name, columns, and column types
	// columnTypes is a map of column name to column type (e.g., "active" -> "BOOLEAN" or "VARCHAR(10)")
	CreateTable(namespace, tableName string, columns []ColumnDef) error

	// DropTable drops the table if it exists
	DropTable(namespace, tableName string) error

	// InsertRow inserts a row into the table
	InsertRow(namespace, tableName string, values map[string]string) error

	// UpdateRow updates a row in the table
	UpdateRow(namespace, tableName, idColumn, idValue string, values map[string]string) error

	// DeleteRow deletes a row from the table
	DeleteRow(namespace, tableName, idColumn, idValue string) error

	// GetHubYAML returns the LynqHub YAML configuration for this datasource
	GetHubYAML(hubName, namespace string, syncInterval string, tableName string, valueMappings, extraValueMappings map[string]string) string

	// ExecSQL executes a raw SQL statement (for advanced operations)
	ExecSQL(namespace, sql string) (string, error)
}

// ColumnDef defines a table column
type ColumnDef struct {
	Name       string
	Type       string
	PrimaryKey bool
	NotNull    bool
	Default    string
}

// MySQLTestAdapter implements TestDatasourceAdapter for MySQL
type MySQLTestAdapter struct {
	Host     string
	Port     int
	Database string
	Username string
	Password string
}

// NewMySQLTestAdapter creates a new MySQL test adapter with default configuration
func NewMySQLTestAdapter() *MySQLTestAdapter {
	return &MySQLTestAdapter{
		Host:     "127.0.0.1",
		Port:     3306,
		Database: "testdb",
		Username: "root",
		Password: "test-password",
	}
}

func (m *MySQLTestAdapter) GetType() string {
	return "mysql"
}

func (m *MySQLTestAdapter) GetServiceHost(namespace string) string {
	return fmt.Sprintf("mysql.%s.svc.cluster.local", namespace)
}

func (m *MySQLTestAdapter) GetDefaultPort() int {
	return 3306
}

func (m *MySQLTestAdapter) CreateTable(namespace, tableName string, columns []ColumnDef) error {
	// Build CREATE TABLE statement
	sql := fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (", tableName)
	for i, col := range columns {
		if i > 0 {
			sql += ", "
		}
		sql += col.Name + " " + col.Type
		if col.NotNull {
			sql += " NOT NULL"
		}
		if col.Default != "" {
			sql += " DEFAULT " + col.Default
		}
		if col.PrimaryKey {
			sql += " PRIMARY KEY"
		}
	}
	sql += ");"

	_, err := m.ExecSQL(namespace, sql)
	return err
}

func (m *MySQLTestAdapter) DropTable(namespace, tableName string) error {
	sql := fmt.Sprintf("DROP TABLE IF EXISTS %s;", tableName)
	_, err := m.ExecSQL(namespace, sql)
	return err
}

func (m *MySQLTestAdapter) InsertRow(namespace, tableName string, values map[string]string) error {
	columns := ""
	vals := ""
	updates := ""
	first := true
	for col, val := range values {
		if !first {
			columns += ", "
			vals += ", "
			updates += ", "
		}
		columns += col
		vals += "'" + val + "'"
		updates += col + "='" + val + "'"
		first = false
	}

	sql := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s) ON DUPLICATE KEY UPDATE %s;",
		tableName, columns, vals, updates)
	_, err := m.ExecSQL(namespace, sql)
	return err
}

func (m *MySQLTestAdapter) UpdateRow(namespace, tableName, idColumn, idValue string, values map[string]string) error {
	updates := ""
	first := true
	for col, val := range values {
		if !first {
			updates += ", "
		}
		updates += col + "='" + val + "'"
		first = false
	}

	sql := fmt.Sprintf("UPDATE %s SET %s WHERE %s='%s';", tableName, updates, idColumn, idValue)
	_, err := m.ExecSQL(namespace, sql)
	return err
}

func (m *MySQLTestAdapter) DeleteRow(namespace, tableName, idColumn, idValue string) error {
	sql := fmt.Sprintf("DELETE FROM %s WHERE %s='%s';", tableName, idColumn, idValue)
	_, err := m.ExecSQL(namespace, sql)
	return err
}

func (m *MySQLTestAdapter) GetHubYAML(hubName, namespace string, syncInterval string, tableName string, valueMappings, extraValueMappings map[string]string) string {
	yaml := fmt.Sprintf(`apiVersion: operator.lynq.sh/v1
kind: LynqHub
metadata:
  name: %s
  namespace: %s
spec:
  source:
    type: mysql
    syncInterval: %s
    mysql:
      host: %s
      port: %d
      database: %s
      table: %s
      username: %s
      passwordRef:
        name: mysql-root-password
        key: password
  valueMappings:
`, hubName, namespace, syncInterval, m.GetServiceHost(namespace), m.GetDefaultPort(), m.Database, tableName, m.Username)

	for k, v := range valueMappings {
		yaml += fmt.Sprintf("    %s: %s\n", k, v)
	}

	if len(extraValueMappings) > 0 {
		yaml += "  extraValueMappings:\n"
		for k, v := range extraValueMappings {
			yaml += fmt.Sprintf("    %s: %s\n", k, v)
		}
	}

	return yaml
}

func (m *MySQLTestAdapter) ExecSQL(namespace, sql string) (string, error) {
	cmd := exec.Command("kubectl", "exec", "-n", namespace, "deployment/mysql", "--",
		"mysql", "-h", m.Host, fmt.Sprintf("-u%s", m.Username), fmt.Sprintf("-p%s", m.Password), m.Database, "-e", sql)
	output, err := utils.Run(cmd)
	return output, err
}

// Helper functions for common test operations using the adapter

// CreateStandardNodesTable creates the standard nodes table used in most tests
func CreateStandardNodesTable(adapter TestDatasourceAdapter, namespace string) error {
	return adapter.CreateTable(namespace, "nodes", []ColumnDef{
		{Name: "id", Type: "VARCHAR(255)", PrimaryKey: true},
		{Name: "active", Type: "BOOLEAN", NotNull: true, Default: "TRUE"},
	})
}

// CreateVarcharActivateTable creates a nodes table with VARCHAR activate column (for truthy value tests)
func CreateVarcharActivateTable(adapter TestDatasourceAdapter, namespace string) error {
	return adapter.CreateTable(namespace, "nodes", []ColumnDef{
		{Name: "id", Type: "VARCHAR(255)", PrimaryKey: true},
		{Name: "active", Type: "VARCHAR(10)", NotNull: true},
	})
}

// RecreateNodesTableWithVarchar drops and recreates the nodes table with VARCHAR activate column
func RecreateNodesTableWithVarchar(adapter TestDatasourceAdapter, namespace string) error {
	if err := adapter.DropTable(namespace, "nodes"); err != nil {
		return err
	}
	return CreateVarcharActivateTable(adapter, namespace)
}

// RecreateNodesTableWithBoolean drops and recreates the nodes table with BOOLEAN activate column
func RecreateNodesTableWithBoolean(adapter TestDatasourceAdapter, namespace string) error {
	if err := adapter.DropTable(namespace, "nodes"); err != nil {
		return err
	}
	return CreateStandardNodesTable(adapter, namespace)
}

// InsertTestNode inserts a test node with the given uid and active status
func InsertTestNode(adapter TestDatasourceAdapter, namespace, uid string, active bool) error {
	activeStr := "0"
	if active {
		activeStr = "1"
	}
	return adapter.InsertRow(namespace, "nodes", map[string]string{
		"id":     uid,
		"active": activeStr,
	})
}

// InsertTestNodeWithValue inserts a test node with the given uid and activate value (string)
func InsertTestNodeWithValue(adapter TestDatasourceAdapter, namespace, uid, activeValue string) error {
	return adapter.InsertRow(namespace, "nodes", map[string]string{
		"id":     uid,
		"active": activeValue,
	})
}

// DeleteTestNode deletes a test node by uid
func DeleteTestNode(adapter TestDatasourceAdapter, namespace, uid string) error {
	return adapter.DeleteRow(namespace, "nodes", "id", uid)
}

// ApplyHub applies a LynqHub configuration
func ApplyHub(adapter TestDatasourceAdapter, hubName, namespace, syncInterval string) error {
	yaml := adapter.GetHubYAML(hubName, namespace, syncInterval, "nodes",
		map[string]string{"uid": "id", "activate": "active"}, nil)
	cmd := exec.Command("kubectl", "apply", "-f", "-")
	cmd.Stdin = utils.StringReader(yaml)
	_, err := utils.Run(cmd)
	return err
}

// ApplyHubWithExtraMappings applies a LynqHub configuration with extra value mappings
func ApplyHubWithExtraMappings(adapter TestDatasourceAdapter, hubName, namespace, syncInterval string, extraMappings map[string]string) error {
	yaml := adapter.GetHubYAML(hubName, namespace, syncInterval, "nodes",
		map[string]string{"uid": "id", "activate": "active"}, extraMappings)
	cmd := exec.Command("kubectl", "apply", "-f", "-")
	cmd.Stdin = utils.StringReader(yaml)
	_, err := utils.Run(cmd)
	return err
}

// Global test adapter instance - change this to use a different datasource
var testDatasource TestDatasourceAdapter = NewMySQLTestAdapter()

// GetTestDatasource returns the current test datasource adapter
func GetTestDatasource() TestDatasourceAdapter {
	return testDatasource
}

// SetTestDatasource sets the test datasource adapter (useful for testing different datasources)
func SetTestDatasource(adapter TestDatasourceAdapter) {
	testDatasource = adapter
}
