package middleware

import (
	"context"
	"fmt"
	"log"
	"slices"
	"strings"
	"time"

	"github.com/Mersad-Moghaddam/shared/config"
	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/redis/go-redis/v9"
)

// JWTClaims represents the claims in a JWT token
type JWTClaims struct {
	UserID    uint   `json:"user_id"`
	Email     string `json:"email"`
	Role      string `json:"role"`
	TokenType string `json:"token_type"` // "access" or "refresh"
	jwt.RegisteredClaims
}

// TokenPair represents access and refresh tokens
type TokenPair struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
	TokenType    string    `json:"token_type"`
}

// JWTMiddleware provides JWT authentication and authorization
type JWTMiddleware struct {
	secretKey       []byte
	accessTokenTTL  time.Duration
	refreshTokenTTL time.Duration
	redisClient     *redis.Client
	issuer          string
	audience        string
	blacklistPrefix string
	refreshPrefix   string
}

// JWTConfig holds JWT middleware configuration
type JWTConfig struct {
	SecretKey       string
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration
	Issuer          string
	Audience        string
}

// NewJWTMiddleware creates a new JWT middleware instance
func NewJWTMiddleware(secretKey string) *JWTMiddleware {
	return NewJWTMiddlewareWithConfig(JWTConfig{
		SecretKey:       secretKey,
		AccessTokenTTL:  15 * time.Minute,   // Short-lived access tokens
		RefreshTokenTTL: 7 * 24 * time.Hour, // 7 days refresh tokens
		Issuer:          "ts-timeline-system",
		Audience:        "ts-timeline-users",
	})
}

// NewJWTMiddlewareWithConfig creates a new JWT middleware with custom configuration
func NewJWTMiddlewareWithConfig(cfg JWTConfig) *JWTMiddleware {
	return &JWTMiddleware{
		secretKey:       []byte(cfg.SecretKey),
		accessTokenTTL:  cfg.AccessTokenTTL,
		refreshTokenTTL: cfg.RefreshTokenTTL,
		redisClient:     config.GetRedisClient(),
		issuer:          cfg.Issuer,
		audience:        cfg.Audience,
		blacklistPrefix: "blacklist:",
		refreshPrefix:   "refresh:",
	}
}

// AuthMiddleware validates JWT tokens and sets user context
func (j *JWTMiddleware) AuthMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Extract token from Authorization header
		authHeader := c.Get("Authorization")
		if authHeader == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error":   "missing authorization header",
				"code":    "MISSING_AUTH_HEADER",
				"message": "Authorization header is required",
			})
		}

		// Check Bearer prefix
		const prefix = "Bearer "
		if len(authHeader) < len(prefix) || !strings.EqualFold(authHeader[:len(prefix)], prefix) {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error":   "invalid authorization format",
				"code":    "INVALID_AUTH_FORMAT",
				"message": "Authorization header must start with 'Bearer '",
			})
		}

		// Extract token
		tokenString := strings.TrimSpace(authHeader[len(prefix):])
		if tokenString == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error":   "empty token",
				"code":    "EMPTY_TOKEN",
				"message": "Token cannot be empty",
			})
		}

		// Validate and parse token
		claims, err := j.ValidateAccessToken(tokenString)
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error":   "invalid token",
				"code":    "INVALID_TOKEN",
				"message": err.Error(),
			})
		}

		// Check if token is blacklisted
		if j.IsTokenBlacklisted(tokenString) {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error":   "token revoked",
				"code":    "TOKEN_REVOKED",
				"message": "Token has been revoked",
			})
		}

		// Set user context (provide both snake_case and camelCase keys for compatibility)
		c.Locals("user_id", claims.UserID)
		c.Locals("userID", claims.UserID)
		c.Locals("user_email", claims.Email)
		c.Locals("user_role", claims.Role)
		c.Locals("token_claims", claims)

		// Add security headers
		c.Set("X-User-ID", fmt.Sprintf("%d", claims.UserID))
		c.Set("X-Content-Type-Options", "nosniff")
		c.Set("X-Frame-Options", "DENY")
		c.Set("X-XSS-Protection", "1; mode=block")

		return c.Next()
	}
}

// RoleMiddleware checks if user has required role
func (j *JWTMiddleware) RoleMiddleware(requiredRoles ...string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		userRole := c.Locals("user_role")
		if userRole == nil {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error":   "no role found",
				"code":    "NO_ROLE",
				"message": "User role not found in token",
			})
		}

		role := userRole.(string)
		for _, requiredRole := range requiredRoles {
			if role == requiredRole || role == "admin" { // Admin can access everything
				return c.Next()
			}
		}

		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error":   "insufficient permissions",
			"code":    "INSUFFICIENT_PERMISSIONS",
			"message": fmt.Sprintf("Required role: %v, got: %s", requiredRoles, role),
		})
	}
}

// GenerateTokenPair creates both access and refresh tokens
func (j *JWTMiddleware) GenerateTokenPair(userID uint, email, role string) (*TokenPair, error) {
	now := time.Now()

	// Generate access token
	accessClaims := &JWTClaims{
		UserID:    userID,
		Email:     email,
		Role:      role,
		TokenType: "access",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(j.accessTokenTTL)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    j.issuer,
			Audience:  []string{j.audience},
			ID:        fmt.Sprintf("%d-%d", userID, now.Unix()),
		},
	}

	accessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims)
	accessTokenString, err := accessToken.SignedString(j.secretKey)
	if err != nil {
		return nil, fmt.Errorf("failed to sign access token: %w", err)
	}

	// Generate refresh token
	refreshClaims := &JWTClaims{
		UserID:    userID,
		Email:     email,
		Role:      role,
		TokenType: "refresh",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(j.refreshTokenTTL)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    j.issuer,
			Audience:  []string{j.audience},
			ID:        fmt.Sprintf("refresh-%d-%d", userID, now.Unix()),
		},
	}

	refreshToken := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims)
	refreshTokenString, err := refreshToken.SignedString(j.secretKey)
	if err != nil {
		return nil, fmt.Errorf("failed to sign refresh token: %w", err)
	}

	// Store refresh token in Redis
	if j.redisClient != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		refreshKey := fmt.Sprintf("%s%d", j.refreshPrefix, userID)
		if err := j.redisClient.Set(ctx, refreshKey, refreshTokenString, j.refreshTokenTTL).Err(); err != nil {
			log.Printf("Failed to store refresh token in Redis: %v", err)
		}
	}

	return &TokenPair{
		AccessToken:  accessTokenString,
		RefreshToken: refreshTokenString,
		ExpiresAt:    now.Add(j.accessTokenTTL),
		TokenType:    "Bearer",
	}, nil
}

// ValidateAccessToken validates and parses an access token
func (j *JWTMiddleware) ValidateAccessToken(tokenString string) (*JWTClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return j.secretKey, nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	claims, ok := token.Claims.(*JWTClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}

	if claims.TokenType != "access" {
		return nil, fmt.Errorf("invalid token type: expected access, got %s", claims.TokenType)
	}

	if !slices.Contains(claims.Audience, j.audience) {
		return nil, fmt.Errorf("invalid audience")
	}

	if claims.Issuer != j.issuer {
		return nil, fmt.Errorf("invalid issuer")
	}

	return claims, nil
}

// RefreshToken generates a new access token using a valid refresh token
func (j *JWTMiddleware) RefreshToken(refreshTokenString string) (*TokenPair, error) {
	// Parse refresh token
	token, err := jwt.ParseWithClaims(refreshTokenString, &JWTClaims{}, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return j.secretKey, nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to parse refresh token: %w", err)
	}

	claims, ok := token.Claims.(*JWTClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid refresh token claims")
	}

	// Verify token type
	if claims.TokenType != "refresh" {
		return nil, fmt.Errorf("invalid token type: expected refresh, got %s", claims.TokenType)
	}

	// Check if refresh token exists in Redis
	if j.redisClient != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		refreshKey := fmt.Sprintf("%s%d", j.refreshPrefix, claims.UserID)
		storedToken, err := j.redisClient.Get(ctx, refreshKey).Result()
		if err == redis.Nil {
			return nil, fmt.Errorf("refresh token not found or expired")
		}
		if err != nil {
			log.Printf("Failed to check refresh token in Redis: %v", err)
		}
		if storedToken != refreshTokenString {
			return nil, fmt.Errorf("invalid refresh token")
		}
	}

	// Generate new token pair
	return j.GenerateTokenPair(claims.UserID, claims.Email, claims.Role)
}

// BlacklistToken adds a token to the blacklist
func (j *JWTMiddleware) BlacklistToken(tokenString string) error {
	if j.redisClient == nil {
		return fmt.Errorf("redis client not available")
	}

	// Parse token to get expiration
	token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, func(token *jwt.Token) (any, error) {
		return j.secretKey, nil
	})

	if err != nil {
		return fmt.Errorf("failed to parse token for blacklisting: %w", err)
	}

	claims, ok := token.Claims.(*JWTClaims)
	if !ok {
		return fmt.Errorf("invalid token claims")
	}

	// Calculate TTL until token expires
	ttl := time.Until(claims.ExpiresAt.Time)
	if ttl <= 0 {
		return nil // Token already expired
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	blacklistKey := fmt.Sprintf("%s%s", j.blacklistPrefix, tokenString)
	return j.redisClient.Set(ctx, blacklistKey, "1", ttl).Err()
}

// IsTokenBlacklisted checks if a token is blacklisted
func (j *JWTMiddleware) IsTokenBlacklisted(tokenString string) bool {
	if j.redisClient == nil {
		return false
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	blacklistKey := fmt.Sprintf("%s%s", j.blacklistPrefix, tokenString)
	_, err := j.redisClient.Get(ctx, blacklistKey).Result()
	return err != redis.Nil
}

// RevokeRefreshToken removes a refresh token from Redis
func (j *JWTMiddleware) RevokeRefreshToken(userID uint) error {
	if j.redisClient == nil {
		return fmt.Errorf("redis client not available")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	refreshKey := fmt.Sprintf("%s%d", j.refreshPrefix, userID)
	return j.redisClient.Del(ctx, refreshKey).Err()
}

// GetUserFromToken extracts user information from a token without validating it
func (j *JWTMiddleware) GetUserFromToken(tokenString string) (*JWTClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, func(token *jwt.Token) (any, error) {
		return j.secretKey, nil
	}, jwt.WithoutClaimsValidation())

	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	claims, ok := token.Claims.(*JWTClaims)
	if !ok {
		return nil, fmt.Errorf("invalid token claims")
	}

	return claims, nil
}

// CleanupExpiredTokens removes expired tokens from blacklist (should be run periodically)
func (j *JWTMiddleware) CleanupExpiredTokens() error {
	if j.redisClient == nil {
		return fmt.Errorf("redis client not available")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Use SCAN to iterate through blacklisted tokens
	iter := j.redisClient.Scan(ctx, 0, j.blacklistPrefix+"*", 100).Iterator()
	var expiredKeys []string

	for iter.Next(ctx) {
		key := iter.Val()

		// Check TTL
		ttl := j.redisClient.TTL(ctx, key)
		if ttl.Val() <= 0 {
			expiredKeys = append(expiredKeys, key)
		}
	}

	if err := iter.Err(); err != nil {
		return fmt.Errorf("failed to scan blacklisted tokens: %w", err)
	}

	// Delete expired keys in batches
	if len(expiredKeys) > 0 {
		if err := j.redisClient.Del(ctx, expiredKeys...).Err(); err != nil {
			return fmt.Errorf("failed to delete expired tokens: %w", err)
		}
		log.Printf("Cleaned up %d expired blacklisted tokens", len(expiredKeys))
	}

	return nil
}

// TokenInfo represents token information for debugging
type TokenInfo struct {
	UserID    uint      `json:"user_id"`
	Email     string    `json:"email"`
	Role      string    `json:"role"`
	TokenType string    `json:"token_type"`
	IssuedAt  time.Time `json:"issued_at"`
	ExpiresAt time.Time `json:"expires_at"`
	Valid     bool      `json:"valid"`
}

// InspectToken returns information about a token for debugging
func (j *JWTMiddleware) InspectToken(tokenString string) (*TokenInfo, error) {
	claims, err := j.GetUserFromToken(tokenString)
	if err != nil {
		return nil, err
	}

	// Check if token is valid (not expired)
	valid := claims.ExpiresAt.After(time.Now()) && !j.IsTokenBlacklisted(tokenString)

	return &TokenInfo{
		UserID:    claims.UserID,
		Email:     claims.Email,
		Role:      claims.Role,
		TokenType: claims.TokenType,
		IssuedAt:  claims.IssuedAt.Time,
		ExpiresAt: claims.ExpiresAt.Time,
		Valid:     valid,
	}, nil
}
