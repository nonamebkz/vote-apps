package repositories

import (
	"errors"
	"fmt"

	"gorm.io/gorm"
)

// BaseRepository defines common CRUD operations interface
type BaseRepository[T any] interface {
	Create(entity *T) error
	GetByID(id uint) (*T, error)
	Update(entity *T) error
	Delete(id uint) error
	List(limit, offset int) ([]*T, error)
	Count() (int64, error)
	Transaction(fn func(*gorm.DB) error) error
}

// BaseRepositoryImpl provides common database operations
type BaseRepositoryImpl[T any] struct {
	db *gorm.DB
}

// NewBaseRepository creates a new base repository
func NewBaseRepository[T any](db *gorm.DB) *BaseRepositoryImpl[T] {
	return &BaseRepositoryImpl[T]{db: db}
}

// Create creates a new entity
func (r *BaseRepositoryImpl[T]) Create(entity *T) error {
	if entity == nil {
		return errors.New("entity cannot be nil")
	}
	
	result := r.db.Create(entity)
	if result.Error != nil {
		return fmt.Errorf("failed to create entity: %w", result.Error)
	}
	
	return nil
}

// GetByID retrieves an entity by ID
func (r *BaseRepositoryImpl[T]) GetByID(id uint) (*T, error) {
	if id == 0 {
		return nil, errors.New("invalid ID: cannot be zero")
	}
	
	var entity T
	result := r.db.First(&entity, id)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("entity with ID %d not found", id)
		}
		return nil, fmt.Errorf("failed to get entity by ID %d: %w", id, result.Error)
	}
	
	return &entity, nil
}

// Update updates an existing entity
func (r *BaseRepositoryImpl[T]) Update(entity *T) error {
	if entity == nil {
		return errors.New("entity cannot be nil")
	}
	
	result := r.db.Save(entity)
	if result.Error != nil {
		return fmt.Errorf("failed to update entity: %w", result.Error)
	}
	
	if result.RowsAffected == 0 {
		return errors.New("no rows affected during update")
	}
	
	return nil
}

// Delete deletes an entity by ID
func (r *BaseRepositoryImpl[T]) Delete(id uint) error {
	if id == 0 {
		return errors.New("invalid ID: cannot be zero")
	}
	
	var entity T
	result := r.db.Delete(&entity, id)
	if result.Error != nil {
		return fmt.Errorf("failed to delete entity with ID %d: %w", id, result.Error)
	}
	
	if result.RowsAffected == 0 {
		return fmt.Errorf("entity with ID %d not found", id)
	}
	
	return nil
}

// List retrieves entities with pagination
func (r *BaseRepositoryImpl[T]) List(limit, offset int) ([]*T, error) {
	if limit < 0 {
		return nil, errors.New("limit cannot be negative")
	}
	if offset < 0 {
		return nil, errors.New("offset cannot be negative")
	}
	
	var entities []*T
	query := r.db.Limit(limit).Offset(offset)
	
	result := query.Find(&entities)
	if result.Error != nil {
		return nil, fmt.Errorf("failed to list entities: %w", result.Error)
	}
	
	return entities, nil
}

// Count returns the total number of entities
func (r *BaseRepositoryImpl[T]) Count() (int64, error) {
	var count int64
	var entity T
	
	result := r.db.Model(&entity).Count(&count)
	if result.Error != nil {
		return 0, fmt.Errorf("failed to count entities: %w", result.Error)
	}
	
	return count, nil
}

// Transaction executes a function within a database transaction
func (r *BaseRepositoryImpl[T]) Transaction(fn func(*gorm.DB) error) error {
	if fn == nil {
		return errors.New("transaction function cannot be nil")
	}
	
	return r.db.Transaction(fn)
}

// GetDB returns the underlying database connection for custom queries
func (r *BaseRepositoryImpl[T]) GetDB() *gorm.DB {
	return r.db
}