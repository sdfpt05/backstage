package repository

import (
	"example.com/backstage/services/truck/internal/database"
)

// Repository provides data access methods
type Repository interface {
	// TODO: Add repository methods here
}

// repo is an implementation of the Repository interface
type repo struct {
	db database.DB
}

// NewRepository creates a new repository instance
func NewRepository(db database.DB) Repository {
	return &repo{
		db: db,
	}
}

// TODO: Implement repository methods
