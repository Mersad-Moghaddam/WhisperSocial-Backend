package config

import (
	"context"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// DB is the global GORM database instance
var DB *gorm.DB

// DBMutex protects database operations during shutdown
var DBMutex sync.RWMutex

// DatabaseConfig holds database configuration
type DatabaseConfig struct {
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
	LogLevel        logger.LogLevel
}

// DefaultDatabaseConfig returns default database configuration
func DefaultDatabaseConfig() *DatabaseConfig {
	return &DatabaseConfig{
		MaxOpenConns:    25,
		MaxIdleConns:    10,
		ConnMaxLifetime: 5 * time.Minute,
		ConnMaxIdleTime: 10 * time.Minute,
		LogLevel:        logger.Warn,
	}
}

// InitDB initializes the database connection using environment variables.
// It also performs auto-migration for the provided model(s) and configures the connection pool.
func InitDB(model any) {
	InitDBWithConfig(model, DefaultDatabaseConfig())
}

// InitDBWithConfig initializes the database with custom configuration
func InitDBWithConfig(model any, config *DatabaseConfig) {
	dsn := buildDSN()

	var err error
	DB, err = gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(config.LogLevel),
		NowFunc: func() time.Time {
			return time.Now().UTC()
		},
		PrepareStmt: true, // Cache prepared statements
	})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	// Perform auto-migration
	if model != nil {
		if err := DB.AutoMigrate(model); err != nil {
			log.Fatalf("Failed to auto-migrate database: %v", err)
		}
	}

	// Configure connection pool
	sqlDB, err := DB.DB()
	if err != nil {
		log.Fatalf("Failed to get underlying sql.DB: %v", err)
	}

	// Set connection pool parameters
	sqlDB.SetMaxOpenConns(config.MaxOpenConns)
	sqlDB.SetMaxIdleConns(config.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(config.ConnMaxLifetime)
	sqlDB.SetConnMaxIdleTime(config.ConnMaxIdleTime)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := sqlDB.PingContext(ctx); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}

	log.Printf("Database connection established successfully with pool config: maxOpen=%d, maxIdle=%d, maxLifetime=%v",
		config.MaxOpenConns, config.MaxIdleConns, config.ConnMaxLifetime)
}

// buildDSN builds the database DSN from environment variables
func buildDSN() string {
	user := os.Getenv("DB_USER")
	password := os.Getenv("DB_PASSWORD")
	host := os.Getenv("DB_HOST")
	port := os.Getenv("DB_PORT")
	name := os.Getenv("DB_NAME")

	// Set defaults if not provided
	if host == "" {
		host = "localhost"
	}
	if port == "" {
		port = "3306"
	}
	if user == "" {
		user = "root"
	}

	return fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=UTC&timeout=10s&readTimeout=30s&writeTimeout=30s&maxAllowedPacket=67108864",
		user, password, host, port, name)
}

// GetDB returns the database instance with read lock
func GetDB() *gorm.DB {
	DBMutex.RLock()
	defer DBMutex.RUnlock()
	return DB
}

// HealthCheck performs a database health check
func HealthCheck() error {
	db := GetDB()
	if db == nil {
		return fmt.Errorf("database connection is nil")
	}

	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("failed to get underlying sql.DB: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := sqlDB.PingContext(ctx); err != nil {
		return fmt.Errorf("database ping failed: %w", err)
	}

	return nil
}

// GetConnectionStats returns database connection statistics
func GetConnectionStats() map[string]any {
	db := GetDB()
	if db == nil {
		return map[string]any{"error": "database connection is nil"}
	}

	sqlDB, err := db.DB()
	if err != nil {
		return map[string]any{"error": fmt.Sprintf("failed to get sql.DB: %v", err)}
	}

	stats := sqlDB.Stats()
	return map[string]any{
		"max_open_connections": stats.MaxOpenConnections,
		"open_connections":     stats.OpenConnections,
		"in_use":               stats.InUse,
		"idle":                 stats.Idle,
		"wait_count":           stats.WaitCount,
		"wait_duration":        stats.WaitDuration.String(),
		"max_idle_closed":      stats.MaxIdleClosed,
		"max_idle_time_closed": stats.MaxIdleTimeClosed,
		"max_lifetime_closed":  stats.MaxLifetimeClosed,
	}
}

// CloseDB closes the database connection gracefully
func CloseDB() error {
	DBMutex.Lock()
	defer DBMutex.Unlock()

	if DB == nil {
		return nil
	}

	sqlDB, err := DB.DB()
	if err != nil {
		return fmt.Errorf("failed to get underlying sql.DB: %w", err)
	}

	log.Println("Closing database connections...")

	// Set a timeout for closing connections
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- sqlDB.Close()
	}()

	select {
	case err := <-done:
		if err != nil {
			log.Printf("Error closing database: %v", err)
			return err
		}
		log.Println("Database connections closed successfully")
		DB = nil
		return nil
	case <-ctx.Done():
		log.Println("Database close timeout exceeded")
		return fmt.Errorf("database close timeout")
	}
}

// Transaction executes a function within a database transaction
func Transaction(fn func(*gorm.DB) error) error {
	db := GetDB()
	if db == nil {
		return fmt.Errorf("database connection is nil")
	}

	return db.Transaction(fn)
}

// WithTimeout creates a database context with timeout
func WithTimeout(timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), timeout)
}
