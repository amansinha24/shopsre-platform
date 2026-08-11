package handler

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/amansinha24/shopsre-platform/auth/internal/domain"
	"github.com/amansinha24/shopsre-platform/auth/internal/repository"
	"github.com/aws/aws-xray-sdk-go/xray"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/redis/go-redis/v9"
)

// Prometheus metrics
// These are automatically scraped by Prometheus every 15 seconds
var (
	loginTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "auth_login_requests_total",
			Help: "Total number of login attempts",
		},
		[]string{"status"}, // label: success or failure
	)

	registerTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "auth_register_requests_total",
			Help: "Total number of registration attempts",
		},
		[]string{"status"},
	)

	requestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "auth_request_duration_seconds",
			Help:    "Duration of auth requests in seconds",
			Buckets: []float64{0.01, 0.05, 0.1, 0.3, 0.5, 1.0, 2.0},
		},
		[]string{"endpoint", "status"},
	)

	activeUsers = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "auth_active_sessions_total",
			Help: "Total number of active user sessions in Redis",
		},
	)
)

// AuthHandler holds everything the handler needs
type AuthHandler struct {
	userRepo    *repository.UserRepository
	redisClient *redis.Client
	jwtSecret   string
}

// NewAuthHandler creates a new AuthHandler
func NewAuthHandler(
	userRepo *repository.UserRepository,
	redisClient *redis.Client,
	jwtSecret string,
) *AuthHandler {
	return &AuthHandler{
		userRepo:    userRepo,
		redisClient: redisClient,
		jwtSecret:   jwtSecret,
	}
}

// Register handles POST /api/auth/register
func (h *AuthHandler) Register(c *gin.Context) {
	// Start timer for Prometheus metric
	start := time.Now()

	// Start X-Ray subsegment for tracing
	ctx, seg := xray.BeginSubsegment(c.Request.Context(), "auth-register")
	if seg != nil {
    	defer seg.Close(nil)
    	_ = ctx
	}

	// Parse request body
	var req domain.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		duration := time.Since(start).Seconds()
		requestDuration.WithLabelValues("register", "error").Observe(duration)
		registerTotal.WithLabelValues("failure").Inc()

		c.JSON(http.StatusBadRequest, domain.ErrorResponse{
			Error:   "invalid_request",
			Message: err.Error(),
		})
		return
	}

	// Add annotation to X-Ray trace
	if seg != nil {
    	seg.AddAnnotation("email", req.Email)
	}

	// Create user object
	user := &domain.User{
		Name:  req.Name,
		Email: req.Email,
	}

	// Save to database
	if err := h.userRepo.Create(user, req.Password); err != nil {
		duration := time.Since(start).Seconds()
		requestDuration.WithLabelValues("register", "error").Observe(duration)
		registerTotal.WithLabelValues("failure").Inc()

		// Email already exists
		if err.Error() == "email already registered" {
			c.JSON(http.StatusConflict, domain.ErrorResponse{
				Error:   "email_exists",
				Message: "This email is already registered",
			})
			return
		}

		c.JSON(http.StatusInternalServerError, domain.ErrorResponse{
			Error:   "server_error",
			Message: "Failed to create account",
		})
		return
	}

	// Record success metrics
	duration := time.Since(start).Seconds()
	requestDuration.WithLabelValues("register", "success").Observe(duration)
	registerTotal.WithLabelValues("success").Inc()

	c.JSON(http.StatusCreated, domain.RegisterResponse{
		UserID:  user.ID,
		Name:    user.Name,
		Email:   user.Email,
		Message: "Account created successfully",
	})
}

// Login handles POST /api/auth/login
func (h *AuthHandler) Login(c *gin.Context) {
	// Start timer for Prometheus metric
	start := time.Now()

	// Start X-Ray subsegment
	ctx, seg := xray.BeginSubsegment(c.Request.Context(), "auth-login")
	if seg != nil {
    	defer seg.Close(nil)
    	_ = ctx
	}

	// Parse request body
	var req domain.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		duration := time.Since(start).Seconds()
		requestDuration.WithLabelValues("login", "error").Observe(duration)
		loginTotal.WithLabelValues("failure").Inc()

		c.JSON(http.StatusBadRequest, domain.ErrorResponse{
			Error:   "invalid_request",
			Message: err.Error(),
		})
		return
	}

	// Add annotation to X-Ray trace
	if seg != nil {
    	seg.AddAnnotation("email", req.Email)
	}

	// Find user in database
	user, hashedPassword, err := h.userRepo.FindByEmail(req.Email)
	if err != nil {
		duration := time.Since(start).Seconds()
		requestDuration.WithLabelValues("login", "error").Observe(duration)
		loginTotal.WithLabelValues("failure").Inc()

		c.JSON(http.StatusUnauthorized, domain.ErrorResponse{
			Error:   "invalid_credentials",
			Message: "Invalid email or password",
		})
		return
	}

	// Verify password
	if !h.userRepo.VerifyPassword(hashedPassword, req.Password) {
		duration := time.Since(start).Seconds()
		requestDuration.WithLabelValues("login", "error").Observe(duration)
		loginTotal.WithLabelValues("failure").Inc()

		c.JSON(http.StatusUnauthorized, domain.ErrorResponse{
			Error:   "invalid_credentials",
			Message: "Invalid email or password",
		})
		return
	}

	// Generate JWT token
	token, err := h.generateJWT(user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, domain.ErrorResponse{
			Error:   "server_error",
			Message: "Failed to generate token",
		})
		return
	}

	// Store session in Redis
	// Key: session:userID → Value: token → Expires: 24 hours
	err = h.storeSession(ctx, user.ID, token)
	if err != nil {
		// Redis failure is not critical — login still works
		// Just log it — we will handle this in CloudWatch
		fmt.Printf("WARNING: failed to store session in Redis: %v\n", err)
	} else {
		// Increment active sessions gauge
		activeUsers.Inc()
	}

	// Record success metrics
	duration := time.Since(start).Seconds()
	requestDuration.WithLabelValues("login", "success").Observe(duration)
	loginTotal.WithLabelValues("success").Inc()

	c.JSON(http.StatusOK, domain.LoginResponse{
		Token:   token,
		UserID:  user.ID,
		Name:    user.Name,
		Email:   user.Email,
		Message: "Login successful",
	})
}

// ValidateToken handles GET /api/auth/validate
// Other services call this to verify a JWT token is valid
func (h *AuthHandler) ValidateToken(c *gin.Context) {
	// Start X-Ray subsegment
	_, seg := xray.BeginSubsegment(c.Request.Context(), "auth-validate")
	if seg != nil {
    	defer seg.Close(nil)
	}

	// Get token from Authorization header
	// Format: "Bearer <token>"
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" || len(authHeader) < 8 {
		c.JSON(http.StatusUnauthorized, domain.ErrorResponse{
			Error:   "missing_token",
			Message: "Authorization header is required",
		})
		return
	}

	tokenString := authHeader[7:] // Remove "Bearer " prefix

	// Parse and validate the JWT token
	claims, err := h.parseJWT(tokenString)
	if err != nil {
		c.JSON(http.StatusUnauthorized, domain.ErrorResponse{
			Error:   "invalid_token",
			Message: "Token is invalid or expired",
		})
		return
	}

	if seg != nil {
    	seg.AddAnnotation("user_id", fmt.Sprintf("%v", claims["user_id"]))
	}

	c.JSON(http.StatusOK, gin.H{
		"valid":   true,
		"user_id": claims["user_id"],
		"email":   claims["email"],
	})
}

// Healthz handles GET /healthz
// Kubernetes uses this to know if the pod is alive
func (h *AuthHandler) Healthz(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "healthy",
		"service": "auth",
	})
}

// generateJWT creates a signed JWT token for a user
func (h *AuthHandler) generateJWT(user *domain.User) (string, error) {
	claims := jwt.MapClaims{
		"user_id": user.ID,
		"email":   user.Email,
		"name":    user.Name,
		// Token expires in 24 hours
		"exp": time.Now().Add(24 * time.Hour).Unix(),
		// Token issued at
		"iat": time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(h.jwtSecret))
}

// parseJWT validates a JWT token and returns its claims
func (h *AuthHandler) parseJWT(tokenString string) (jwt.MapClaims, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(h.jwtSecret), nil
	})

	if err != nil || !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, fmt.Errorf("invalid claims")
	}

	return claims, nil
}

// storeSession saves the JWT token in Redis
func (h *AuthHandler) storeSession(ctx context.Context, userID int64, token string) error {
	key := fmt.Sprintf("session:%d", userID)
	return h.redisClient.Set(ctx, key, token, 24*time.Hour).Err()
}