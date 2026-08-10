package domain

import "time"

// User represents a user in our system
type User struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Password  string    `json:"-"` // never send password in JSON response
	CreatedAt time.Time `json:"created_at"`
}

// RegisterRequest is what the frontend sends when a user signs up
type RegisterRequest struct {
	Name     string `json:"name"     binding:"required"`
	Email    string `json:"email"    binding:"required,email"`
	Password string `json:"password" binding:"required,min=6"`
}

// LoginRequest is what the frontend sends when a user logs in
type LoginRequest struct {
	Email    string `json:"email"    binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

// LoginResponse is what we send back to the frontend after login
type LoginResponse struct {
	Token   string `json:"token"`
	UserID  int64  `json:"user_id"`
	Name    string `json:"name"`
	Email   string `json:"email"`
	Message string `json:"message"`
}

// RegisterResponse is what we send back after successful registration
type RegisterResponse struct {
	UserID  int64  `json:"user_id"`
	Name    string `json:"name"`
	Email   string `json:"email"`
	Message string `json:"message"`
}

// ErrorResponse is what we send back when something goes wrong
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}