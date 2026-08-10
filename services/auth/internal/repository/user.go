package repository

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/amansinha24/shopsre-platform/auth/internal/domain"
	_ "github.com/lib/pq"
	"golang.org/x/crypto/bcrypt"
)

// UserRepository handles all database operations for users
type UserRepository struct {
	db *sql.DB
}

// NewUserRepository creates a new UserRepository
func NewUserRepository(db *sql.DB) *UserRepository {
	return &UserRepository{db: db}
}

// CreateTable creates the users table if it does not exist
// This runs automatically when the service starts
func (r *UserRepository) CreateTable() error {
	query := `
		CREATE TABLE IF NOT EXISTS users (
			id         SERIAL PRIMARY KEY,
			name       VARCHAR(255) NOT NULL,
			email      VARCHAR(255) UNIQUE NOT NULL,
			password   VARCHAR(255) NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`
	_, err := r.db.Exec(query)
	if err != nil {
		return fmt.Errorf("failed to create users table: %w", err)
	}
	return nil
}

// Create saves a new user to the database
// It hashes the password before saving — never store plain text passwords
func (r *UserRepository) Create(user *domain.User, plainPassword string) error {
	// Hash the password using bcrypt
	// Cost of 12 means it takes ~250ms to hash — slow enough to prevent brute force
	hashedPassword, err := bcrypt.GenerateFromPassword(
		[]byte(plainPassword), 12,
	)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	query := `
		INSERT INTO users (name, email, password)
		VALUES ($1, $2, $3)
		RETURNING id, created_at
	`

	err = r.db.QueryRow(query, user.Name, user.Email, string(hashedPassword)).
		Scan(&user.ID, &user.CreatedAt)
	if err != nil {
		// Check if email already exists
		if err.Error() == `pq: duplicate key value violates unique constraint "users_email_key"` {
			return errors.New("email already registered")
		}
		return fmt.Errorf("failed to create user: %w", err)
	}

	return nil
}

// FindByEmail finds a user by their email address
// Returns the user with hashed password so we can verify login
func (r *UserRepository) FindByEmail(email string) (*domain.User, string, error) {
	query := `
		SELECT id, name, email, password, created_at
		FROM users
		WHERE email = $1
	`

	var user domain.User
	var hashedPassword string

	err := r.db.QueryRow(query, email).Scan(
		&user.ID,
		&user.Name,
		&user.Email,
		&hashedPassword,
		&user.CreatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, "", errors.New("user not found")
		}
		return nil, "", fmt.Errorf("failed to find user: %w", err)
	}

	return &user, hashedPassword, nil
}

// VerifyPassword checks if the plain text password matches the hashed password
func (r *UserRepository) VerifyPassword(hashedPassword, plainPassword string) bool {
	err := bcrypt.CompareHashAndPassword(
		[]byte(hashedPassword),
		[]byte(plainPassword),
	)
	return err == nil
}

// NewDBConnection creates a connection to PostgreSQL
func NewDBConnection(host, port, user, password, dbname string) (*sql.DB, error) {
	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		host, port, user, password, dbname,
	)

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Test the connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Connection pool settings
	// Maximum 25 open connections at once
	db.SetMaxOpenConns(25)
	// Maximum 25 idle connections kept ready
	db.SetMaxIdleConns(25)

	return db, nil
}