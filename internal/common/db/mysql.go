package db

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

// MySQLConfig holds the configuration for MySQL connection pool
type MySQLConfig struct {
	// DSN is the data source name
	// Format: "user:password@tcp(host:port)/dbname?parseTime=true&loc=Local"
	DSN string

	// MaxOpenConnections is the maximum number of open connections to the database
	// Default: 25
	MaxOpenConnections int

	// MaxIdleConnections is the maximum number of connections in the idle connection pool
	// Default: 5
	MaxIdleConnections int

	// ConnMaxLifetime is the maximum amount of time a connection may be reused
	// Default: 5 minutes
	ConnMaxLifetime time.Duration

	// ConnMaxIdleTime is the maximum amount of time a connection may be idle
	// Default: 10 minutes
	ConnMaxIdleTime time.Duration
}

// DefaultMySQLConfig returns the default MySQL configuration
func DefaultMySQLConfig() *MySQLConfig {
	return &MySQLConfig{
		MaxOpenConnections: 25,
		MaxIdleConnections: 5,
		ConnMaxLifetime:    5 * time.Minute,
		ConnMaxIdleTime:    10 * time.Minute,
	}
}

// MySQL implements the Database interface using MySQL driver with connection pooling
// Each MySQL instance manages its own connection pool
type MySQL struct {
	db     *sql.DB
	config *MySQLConfig
	mu     sync.RWMutex
}

// NewMySQL creates a new MySQL database connection with connection pool
// DSN format: "user:password@tcp(host:port)/dbname?parseTime=true&loc=Local"
func NewMySQL(dsn string) (*MySQL, error) {
	config := DefaultMySQLConfig()
	config.DSN = dsn
	return NewMySQLWithConfig(config)
}

// NewMySQLWithConfig creates a new MySQL database connection with custom configuration
func NewMySQLWithConfig(config *MySQLConfig) (*MySQL, error) {
	if config == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	if config.DSN == "" {
		return nil, fmt.Errorf("DSN cannot be empty")
	}

	// Set defaults if not specified
	if config.MaxOpenConnections == 0 {
		config.MaxOpenConnections = 25
	}
	if config.MaxIdleConnections == 0 {
		config.MaxIdleConnections = 5
	}
	if config.ConnMaxLifetime == 0 {
		config.ConnMaxLifetime = 5 * time.Minute
	}
	if config.ConnMaxIdleTime == 0 {
		config.ConnMaxIdleTime = 10 * time.Minute
	}

	db, err := sql.Open("mysql", config.DSN)
	if err != nil {
		return nil, fmt.Errorf("failed to open database connection: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(config.MaxOpenConnections)
	db.SetMaxIdleConns(config.MaxIdleConnections)
	db.SetConnMaxLifetime(config.ConnMaxLifetime)
	db.SetConnMaxIdleTime(config.ConnMaxIdleTime)

	// Verify the connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &MySQL{db: db, config: config}, nil
}

// NewMySQLWithDB creates a MySQL instance from an existing sql.DB
func NewMySQLWithDB(db *sql.DB) (*MySQL, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}
	return &MySQL{db: db, config: DefaultMySQLConfig()}, nil
}

// GetConfig returns the current MySQL configuration
func (m *MySQL) GetConfig() *MySQLConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config
}

// SetMaxOpenConnections sets the maximum number of open connections
func (m *MySQL) SetMaxOpenConnections(n int) {
	if n > 0 {
		m.mu.Lock()
		defer m.mu.Unlock()
		m.db.SetMaxOpenConns(n)
		m.config.MaxOpenConnections = n
	}
}

// SetMaxIdleConnections sets the maximum number of idle connections
func (m *MySQL) SetMaxIdleConnections(n int) {
	if n >= 0 {
		m.mu.Lock()
		defer m.mu.Unlock()
		m.db.SetMaxIdleConns(n)
		m.config.MaxIdleConnections = n
	}
}

// SetConnMaxLifetime sets the maximum lifetime of a connection
func (m *MySQL) SetConnMaxLifetime(d time.Duration) {
	if d >= 0 {
		m.mu.Lock()
		defer m.mu.Unlock()
		m.db.SetConnMaxLifetime(d)
		m.config.ConnMaxLifetime = d
	}
}

// SetConnMaxIdleTime sets the maximum idle time of a connection
func (m *MySQL) SetConnMaxIdleTime(d time.Duration) {
	if d >= 0 {
		m.mu.Lock()
		defer m.mu.Unlock()
		m.db.SetConnMaxIdleTime(d)
		m.config.ConnMaxIdleTime = d
	}
}

// Query executes a query that returns rows
func (m *MySQL) Query(ctx context.Context, query string, args ...interface{}) (Rows, error) {
	rows, err := m.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	return &MySQLRows{rows: rows}, nil
}

// QueryRow executes a query that returns at most one row
func (m *MySQL) QueryRow(ctx context.Context, query string, args ...interface{}) Row {
	return &MySQLRow{row: m.db.QueryRowContext(ctx, query, args...)}
}

// Exec executes a query that doesn't return rows
func (m *MySQL) Exec(ctx context.Context, query string, args ...interface{}) (Result, error) {
	result, err := m.db.ExecContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("exec failed: %w", err)
	}
	return &MySQLResult{result: result}, nil
}

// Transaction executes a function within a database transaction
func (m *MySQL) Transaction(ctx context.Context, fn func(tx Transaction) error) error {
	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction failed: %w", err)
	}

	myTx := &MySQLTransaction{tx: tx}
	if err := fn(myTx); err != nil {
		_ = myTx.Rollback()
		return err
	}

	return myTx.Commit()
}

// BeginTx starts a new transaction with the given options
func (m *MySQL) BeginTx(ctx context.Context, opts *TxOptions) (Transaction, error) {
	sqlOpts := ConvertTxOptions(opts)
	tx, err := m.db.BeginTx(ctx, sqlOpts)
	if err != nil {
		return nil, fmt.Errorf("begin transaction failed: %w", err)
	}
	return &MySQLTransaction{tx: tx}, nil
}

// Prepare creates a prepared statement for later queries or executions
func (m *MySQL) Prepare(ctx context.Context, query string) (Stmt, error) {
	stmt, err := m.db.PrepareContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("prepare statement failed: %w", err)
	}
	return &MySQLStmt{stmt: stmt}, nil
}

// Ping verifies a connection to the database is still alive
func (m *MySQL) Ping(ctx context.Context) error {
	if err := m.db.PingContext(ctx); err != nil {
		return fmt.Errorf("ping failed: %w", err)
	}
	return nil
}

// Close closes the database connection
func (m *MySQL) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.db.Close(); err != nil {
		return fmt.Errorf("close failed: %w", err)
	}
	return nil
}

// Stats returns database statistics
func (m *MySQL) Stats() Stats {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return ConvertSQLStats(m.db.Stats())
}

// GetDB returns the underlying database instance
func (m *MySQL) GetDB() interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.db
}

// MySQLRows implements the Rows interface
type MySQLRows struct {
	rows *sql.Rows
}

// Next prepares the next result row
func (r *MySQLRows) Next() bool {
	return r.rows.Next()
}

// Scan copies the columns from the current row into the values
func (r *MySQLRows) Scan(dest ...interface{}) error {
	if err := r.rows.Scan(dest...); err != nil {
		return fmt.Errorf("scan failed: %w", err)
	}
	return nil
}

// Close closes the Rows
func (r *MySQLRows) Close() error {
	if err := r.rows.Close(); err != nil {
		return fmt.Errorf("close rows failed: %w", err)
	}
	return nil
}

// Err returns the error encountered during iteration
func (r *MySQLRows) Err() error {
	return r.rows.Err()
}

// Columns returns the column names
func (r *MySQLRows) Columns() ([]string, error) {
	cols, err := r.rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("get columns failed: %w", err)
	}
	return cols, nil
}

// ColumnTypes returns column type information
func (r *MySQLRows) ColumnTypes() ([]ColumnType, error) {
	types, err := r.rows.ColumnTypes()
	if err != nil {
		return nil, fmt.Errorf("get column types failed: %w", err)
	}
	result := make([]ColumnType, len(types))
	for i, t := range types {
		result[i] = &MySQLColumnType{ct: t}
	}
	return result, nil
}

// NextResultSet advances to the next result set
func (r *MySQLRows) NextResultSet() bool {
	return r.rows.NextResultSet()
}

// MySQLRow implements the Row interface
type MySQLRow struct {
	row *sql.Row
}

// Scan copies the columns from the matched row
func (r *MySQLRow) Scan(dest ...interface{}) error {
	if err := r.row.Scan(dest...); err != nil {
		return fmt.Errorf("scan failed: %w", err)
	}
	return nil
}

// MySQLResult implements the Result interface
type MySQLResult struct {
	result sql.Result
}

// LastInsertId returns the last inserted ID
func (r *MySQLResult) LastInsertId() (int64, error) {
	id, err := r.result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("get last insert id failed: %w", err)
	}
	return id, nil
}

// RowsAffected returns the number of rows affected
func (r *MySQLResult) RowsAffected() (int64, error) {
	affected, err := r.result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("get rows affected failed: %w", err)
	}
	return affected, nil
}

// MySQLTransaction implements the Transaction interface
type MySQLTransaction struct {
	tx *sql.Tx
}

// Query executes a query within the transaction
func (t *MySQLTransaction) Query(ctx context.Context, query string, args ...interface{}) (Rows, error) {
	rows, err := t.tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("transaction query failed: %w", err)
	}
	return &MySQLRows{rows: rows}, nil
}

// QueryRow executes a query that returns at most one row
func (t *MySQLTransaction) QueryRow(ctx context.Context, query string, args ...interface{}) Row {
	return &MySQLRow{row: t.tx.QueryRowContext(ctx, query, args...)}
}

// Exec executes a query within the transaction
func (t *MySQLTransaction) Exec(ctx context.Context, query string, args ...interface{}) (Result, error) {
	result, err := t.tx.ExecContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("transaction exec failed: %w", err)
	}
	return &MySQLResult{result: result}, nil
}

// Prepare creates a prepared statement within the transaction
func (t *MySQLTransaction) Prepare(ctx context.Context, query string) (Stmt, error) {
	stmt, err := t.tx.PrepareContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("transaction prepare failed: %w", err)
	}
	return &MySQLStmt{stmt: stmt}, nil
}

// Commit commits the transaction
func (t *MySQLTransaction) Commit() error {
	if err := t.tx.Commit(); err != nil {
		return fmt.Errorf("commit failed: %w", err)
	}
	return nil
}

// Rollback rolls back the transaction
func (t *MySQLTransaction) Rollback() error {
	if err := t.tx.Rollback(); err != nil {
		return fmt.Errorf("rollback failed: %w", err)
	}
	return nil
}

// MySQLStmt implements the Stmt interface
type MySQLStmt struct {
	stmt *sql.Stmt
}

// Exec executes a prepared statement
func (s *MySQLStmt) Exec(ctx context.Context, args ...interface{}) (Result, error) {
	result, err := s.stmt.ExecContext(ctx, args...)
	if err != nil {
		return nil, fmt.Errorf("statement exec failed: %w", err)
	}
	return &MySQLResult{result: result}, nil
}

// Query executes a prepared query statement
func (s *MySQLStmt) Query(ctx context.Context, args ...interface{}) (Rows, error) {
	rows, err := s.stmt.QueryContext(ctx, args...)
	if err != nil {
		return nil, fmt.Errorf("statement query failed: %w", err)
	}
	return &MySQLRows{rows: rows}, nil
}

// QueryRow executes a prepared query that returns at most one row
func (s *MySQLStmt) QueryRow(ctx context.Context, args ...interface{}) Row {
	return &MySQLRow{row: s.stmt.QueryRowContext(ctx, args...)}
}

// Close closes the statement
func (s *MySQLStmt) Close() error {
	if err := s.stmt.Close(); err != nil {
		return fmt.Errorf("close statement failed: %w", err)
	}
	return nil
}

// MySQLColumnType implements the ColumnType interface
type MySQLColumnType struct {
	ct *sql.ColumnType
}

// Name returns the column name
func (c *MySQLColumnType) Name() string {
	return c.ct.Name()
}

// DatabaseTypeName returns the database system type name
func (c *MySQLColumnType) DatabaseTypeName() string {
	return c.ct.DatabaseTypeName()
}

// Length returns the column length
func (c *MySQLColumnType) Length() (int64, bool) {
	return c.ct.Length()
}

// Nullable returns whether the column may be null
func (c *MySQLColumnType) Nullable() (bool, bool) {
	return c.ct.Nullable()
}

// DecimalSize returns the scale and precision
func (c *MySQLColumnType) DecimalSize() (int64, int64, bool) {
	return c.ct.DecimalSize()
}

// ScanType returns a Go type suitable for scanning
func (c *MySQLColumnType) ScanType() interface{} {
	return c.ct.ScanType()
}
