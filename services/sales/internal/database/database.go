package database

import (
	"fmt"
	"time"
	
	"example.com/backstage/services/sales/config"
	
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// DB is an interface for database operations
type DB interface {
	DB() (*gorm.DB, error)
	Close() error
}

// GormDatabase implements the DB interface for GORM
type GormDatabase struct {
	db *gorm.DB
}

// Connect establishes a connection to the database
func Connect(cfg config.DatabaseConfig) (DB, error) {
	dsn := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.DBName, cfg.SSLMode,
	)
	
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}
	
	// Configure connection pool
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get DB instance: %w", err)
	}
	
	// SetMaxIdleConns sets the maximum number of connections in the idle connection pool
	sqlDB.SetMaxIdleConns(10)
	
	// SetMaxOpenConns sets the maximum number of open connections to the database
	sqlDB.SetMaxOpenConns(100)
	
	// SetConnMaxLifetime sets the maximum amount of time a connection may be reused
	sqlDB.SetConnMaxLifetime(time.Hour)
	
	return &GormDatabase{db: db}, nil
}

// DB returns the underlying gorm.DB instance
func (d *GormDatabase) DB() (*gorm.DB, error) {
	return d.db, nil
}

// Close closes the database connection
func (d *GormDatabase) Close() error {
	sqlDB, err := d.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// AutoMigrate runs database migrations
func AutoMigrate(db DB) error {
	gormDB, err := db.DB()
	if err != nil {
		return err
	}
	
	// Add your models to migrate here
	// Example: return gormDB.AutoMigrate(&models.User{}, &models.Product{})
	return nil
}
