package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"
)

// SecurityConfig holds security middleware configuration
type SecurityConfig struct {
	// CSRF protection
	CSRFProtection   bool
	CSRFTokenLength  int
	CSRFCookieName   string
	CSRFHeaderName   string
	CSRFTokenTimeout time.Duration

	// XSS protection
	XSSProtection bool

	// Content Security Policy
	CSPEnabled bool
	CSPPolicy  string

	// Input sanitization
	SanitizeInput bool
	MaxInputSize  int64

	// SQL Injection protection
	SQLInjectionProtection bool

	// Request ID generation
	GenerateRequestID bool

	// Security headers
	SecurityHeaders bool

	// Audit logging
	AuditLogging bool
}

// DefaultSecurityConfig returns default security configuration
func DefaultSecurityConfig() *SecurityConfig {
	return &SecurityConfig{
		CSRFProtection:         true,
		CSRFTokenLength:        32,
		CSRFCookieName:         "_csrf_token",
		CSRFHeaderName:         "X-CSRF-Token",
		CSRFTokenTimeout:       24 * time.Hour,
		XSSProtection:          true,
		CSPEnabled:             true,
		CSPPolicy:              "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' data: https:; font-src 'self' data:;",
		SanitizeInput:          true,
		MaxInputSize:           10 * 1024 * 1024, // 10MB
		SQLInjectionProtection: true,
		GenerateRequestID:      true,
		SecurityHeaders:        true,
		AuditLogging:           true,
	}
}

// SecurityMiddleware provides comprehensive security features
type SecurityMiddleware struct {
	config      *SecurityConfig
	redisClient *redis.Client
}

// NewSecurityMiddleware creates a new security middleware
func NewSecurityMiddleware(config *SecurityConfig, redisClient *redis.Client) *SecurityMiddleware {
	if config == nil {
		config = DefaultSecurityConfig()
	}

	return &SecurityMiddleware{
		config:      config,
		redisClient: redisClient,
	}
}

// SecurityMiddleware returns the main security middleware handler
func (sm *SecurityMiddleware) SecurityMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Generate request ID
		if sm.config.GenerateRequestID {
			requestID := generateSecurityRequestID()
			c.Locals("request_id", requestID)
			c.Set("X-Request-ID", requestID)
		}

		// Add security headers
		if sm.config.SecurityHeaders {
			sm.addSecurityHeaders(c)
		}

		// XSS Protection
		if sm.config.XSSProtection {
			if err := sm.validateXSS(c); err != nil {
				return sm.securityError(c, "XSS_DETECTED", err.Error(), 400)
			}
		}

		// SQL Injection Protection
		if sm.config.SQLInjectionProtection {
			if err := sm.validateSQLInjection(c); err != nil {
				return sm.securityError(c, "SQL_INJECTION_DETECTED", err.Error(), 400)
			}
		}

		// Input Size Validation
		if sm.config.MaxInputSize > 0 {
			if int64(c.Request().Header.ContentLength()) > sm.config.MaxInputSize {
				return sm.securityError(c, "PAYLOAD_TOO_LARGE", "Request payload too large", 413)
			}
		}

		// Input Sanitization
		if sm.config.SanitizeInput {
			sm.sanitizeInput(c)
		}

		// Audit Logging
		if sm.config.AuditLogging {
			sm.logSecurityEvent(c, "REQUEST", "Incoming request processed")
		}

		return c.Next()
	}
}

// CSRFMiddleware provides CSRF protection
func (sm *SecurityMiddleware) CSRFMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		if !sm.config.CSRFProtection {
			return c.Next()
		}

		method := c.Method()

		// Skip CSRF for safe methods
		if method == "GET" || method == "HEAD" || method == "OPTIONS" {
			return c.Next()
		}

		// Get token from header
		token := c.Get(sm.config.CSRFHeaderName)
		if token == "" {
			return sm.securityError(c, "CSRF_TOKEN_MISSING", "CSRF token is required", 403)
		}

		// Validate token
		if !sm.validateCSRFToken(token) {
			return sm.securityError(c, "CSRF_TOKEN_INVALID", "Invalid CSRF token", 403)
		}

		return c.Next()
	}
}

// RBACMiddleware provides Role-Based Access Control
func RBACMiddleware(requiredPermissions ...string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Get user role from context
		userRole := c.Locals("user_role")
		if userRole == nil {
			return fiber.NewError(403, "No role found in token")
		}

		role := userRole.(string)

		// Check if user has required permissions
		if !hasPermissions(role, requiredPermissions) {
			// Log unauthorized access attempt
			userID := c.Locals("user_id")
			log.Printf("RBAC: Unauthorized access attempt - User ID: %v, Role: %s, Required: %v, Path: %s, IP: %s",
				userID, role, requiredPermissions, c.Path(), c.IP())

			return c.Status(403).JSON(fiber.Map{
				"error":   "insufficient_permissions",
				"code":    "INSUFFICIENT_PERMISSIONS",
				"message": fmt.Sprintf("Required permissions: %v", requiredPermissions),
			})
		}

		// Log authorized access
		c.Locals("authorized_permissions", requiredPermissions)
		return c.Next()
	}
}

// addSecurityHeaders adds security headers to the response
func (sm *SecurityMiddleware) addSecurityHeaders(c *fiber.Ctx) {
	// Prevent MIME type sniffing
	c.Set("X-Content-Type-Options", "nosniff")

	// Prevent clickjacking
	c.Set("X-Frame-Options", "DENY")

	// XSS Protection
	c.Set("X-XSS-Protection", "1; mode=block")

	// Referrer Policy
	c.Set("Referrer-Policy", "strict-origin-when-cross-origin")

	// Content Security Policy
	if sm.config.CSPEnabled {
		c.Set("Content-Security-Policy", sm.config.CSPPolicy)
	}

	// Strict Transport Security (HTTPS only)
	if c.Protocol() == "https" {
		c.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
	}

	// Permissions Policy
	c.Set("Permissions-Policy", "geolocation=(), microphone=(), camera=()")

	// Remove server information
	c.Set("Server", "")
}

// validateXSS checks for XSS attacks in request data
func (sm *SecurityMiddleware) validateXSS(c *fiber.Ctx) error {
	// XSS patterns to detect
	xssPatterns := []string{
		`<script[^>]*>.*?</script>`,
		`javascript:`,
		`on\w+\s*=`,
		`<iframe[^>]*>.*?</iframe>`,
		`<object[^>]*>.*?</object>`,
		`<embed[^>]*>`,
		`<link[^>]*>`,
		`<meta[^>]*>`,
		`vbscript:`,
		`data:text/html`,
		`expression\s*\(`,
	}

	// Check URL parameters
	c.Request().URI().QueryArgs().VisitAll(func(key, value []byte) {
		if containsSuspiciousContent(string(value), xssPatterns) {
			sm.logSecurityEvent(c, "XSS_ATTEMPT", fmt.Sprintf("XSS detected in query parameter: %s", string(key)))
		}
	})

	// Check request body for POST/PUT requests
	if c.Method() == "POST" || c.Method() == "PUT" || c.Method() == "PATCH" {
		body := c.Body()
		if containsSuspiciousContent(string(body), xssPatterns) {
			return fmt.Errorf("XSS content detected in request body")
		}
	}

	// Check headers
	suspiciousHeaders := []string{"User-Agent", "Referer", "X-Forwarded-For"}
	for _, header := range suspiciousHeaders {
		value := c.Get(header)
		if containsSuspiciousContent(value, xssPatterns) {
			return fmt.Errorf("XSS content detected in header: %s", header)
		}
	}

	return nil
}

// validateSQLInjection checks for SQL injection attempts
func (sm *SecurityMiddleware) validateSQLInjection(c *fiber.Ctx) error {
	// SQL injection patterns
	sqlPatterns := []string{
		`(?i)\b(union|select|insert|update|delete|drop|create|alter|exec|execute)\b.*\b(from|where|into|values|table|database|schema)\b`,
		`(?i)(\band\b|\bor\b)\s+\d+\s*=\s*\d+`,
		`(?i)(\band\b|\bor\b)\s+['"].*['"]`,
		`(?i)\b(sleep|benchmark|waitfor|delay)\s*\(`,
		`(?i)\b(load_file|into\s+outfile|into\s+dumpfile)\b`,
		`(?i)(--|#|\/\*|\*\/)`,
		`(?i)\b(information_schema|mysql|sys|performance_schema)\b`,
		`(?i)\b(0x[0-9a-f]+|char\(|ascii\(|hex\()\b`,
	}

	// Check URL parameters
	c.Request().URI().QueryArgs().VisitAll(func(key, value []byte) {
		if containsSuspiciousContent(string(value), sqlPatterns) {
			sm.logSecurityEvent(c, "SQL_INJECTION_ATTEMPT", fmt.Sprintf("SQL injection detected in parameter: %s", string(key)))
		}
	})

	// Check request body
	if c.Method() == "POST" || c.Method() == "PUT" || c.Method() == "PATCH" {
		body := c.Body()
		if containsSuspiciousContent(string(body), sqlPatterns) {
			return fmt.Errorf("SQL injection pattern detected in request body")
		}
	}

	return nil
}

// sanitizeInput sanitizes user input
func (sm *SecurityMiddleware) sanitizeInput(c *fiber.Ctx) {
	// Sanitize query parameters
	c.Request().URI().QueryArgs().VisitAll(func(key, value []byte) {
		sanitized := sanitizeString(string(value))
		c.Request().URI().QueryArgs().Set(string(key), sanitized)
	})

	// Note: Body sanitization would require parsing JSON/form data
	// This is typically done at the application layer with proper parsing
}

// validateCSRFToken validates CSRF token
func (sm *SecurityMiddleware) validateCSRFToken(token string) bool {
	if sm.redisClient == nil {
		return false
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	key := fmt.Sprintf("csrf_token:%s", token)
	_, err := sm.redisClient.Get(ctx, key).Result()
	return err == nil
}

// GenerateCSRFToken generates a new CSRF token
func (sm *SecurityMiddleware) GenerateCSRFToken() (string, error) {
	token, err := generateRandomToken(sm.config.CSRFTokenLength)
	if err != nil {
		return "", err
	}

	if sm.redisClient != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		key := fmt.Sprintf("csrf_token:%s", token)
		sm.redisClient.Set(ctx, key, "1", sm.config.CSRFTokenTimeout)
	}

	return token, nil
}

// containsSuspiciousContent checks if content contains suspicious patterns
func containsSuspiciousContent(content string, patterns []string) bool {
	content = strings.ToLower(content)

	for _, pattern := range patterns {
		matched, _ := regexp.MatchString(pattern, content)
		if matched {
			return true
		}
	}

	return false
}

// sanitizeString sanitizes a string by removing potentially dangerous characters
func sanitizeString(s string) string {
	// Remove null bytes
	s = strings.ReplaceAll(s, "\x00", "")

	// Remove control characters except tab, newline, and carriage return
	result := make([]rune, 0, len(s))
	for _, r := range s {
		if r >= 32 || r == '\t' || r == '\n' || r == '\r' {
			result = append(result, r)
		}
	}

	return string(result)
}

// hasPermissions checks if a role has the required permissions
func hasPermissions(userRole string, requiredPermissions []string) bool {
	// Define role permissions
	rolePermissions := map[string][]string{
		"admin": {
			"*", // Admin has all permissions
		},
		"moderator": {
			"posts:read", "posts:write", "posts:delete",
			"users:read", "users:moderate",
			"notifications:read", "notifications:write",
			"timeline:read",
		},
		"premium_user": {
			"posts:read", "posts:write", "posts:update",
			"timeline:read", "timeline:write",
			"notifications:read",
			"follows:read", "follows:write",
		},
		"user": {
			"posts:read", "posts:write",
			"timeline:read",
			"notifications:read",
			"follows:read", "follows:write",
		},
		"guest": {
			"posts:read",
		},
	}

	userPerms, exists := rolePermissions[userRole]
	if !exists {
		return false
	}

	// Check if user has admin privileges

	if slices.Contains(userPerms, "*") {
		return true
	}

	for _, required := range requiredPermissions {
		if !slices.Contains(userPerms, required) {
			return false
		}
	}

	return true

}

// generateSecurityRequestID generates a unique request ID for security middleware
func generateSecurityRequestID() string {
	timestamp := time.Now().UnixNano()
	randomBytes := make([]byte, 8)
	rand.Read(randomBytes)
	return fmt.Sprintf("%d-%s", timestamp, hex.EncodeToString(randomBytes))
}

// generateRandomToken generates a random token
func generateRandomToken(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// securityError creates a standardized security error response
func (sm *SecurityMiddleware) securityError(c *fiber.Ctx, code, message string, status int) error {
	// Log security incident
	sm.logSecurityEvent(c, code, message)

	return c.Status(status).JSON(fiber.Map{
		"error":      true,
		"code":       code,
		"message":    message,
		"timestamp":  time.Now().Unix(),
		"request_id": c.Locals("request_id"),
	})
}

// logSecurityEvent logs security-related events
func (sm *SecurityMiddleware) logSecurityEvent(c *fiber.Ctx, eventType, message string) {
	if !sm.config.AuditLogging {
		return
	}

	requestID := c.Locals("request_id")
	userID := c.Locals("user_id")

	log.Printf("SECURITY_EVENT: Type=%s, RequestID=%v, UserID=%v, IP=%s, Path=%s, Method=%s, Message=%s",
		eventType, requestID, userID, c.IP(), c.Path(), c.Method(), message)

	// Store in Redis for analysis if available
	if sm.redisClient != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		eventKey := fmt.Sprintf("security_event:%d", time.Now().UnixNano())
		eventData := map[string]any{
			"type":       eventType,
			"message":    message,
			"request_id": requestID,
			"user_id":    userID,
			"ip":         c.IP(),
			"path":       c.Path(),
			"method":     c.Method(),
			"timestamp":  time.Now().Unix(),
			"user_agent": c.Get("User-Agent"),
		}

		sm.redisClient.HMSet(ctx, eventKey, eventData)
		sm.redisClient.Expire(ctx, eventKey, 7*24*time.Hour) // Keep for 7 days
	}
}

// InputValidationMiddleware validates common input patterns
func InputValidationMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Validate common parameters
		if id := c.Params("id"); id != "" {
			if !isValidID(id) {
				return c.Status(400).JSON(fiber.Map{
					"error":   "invalid_id",
					"message": "Invalid ID format",
				})
			}
		}

		// Validate query parameters
		if limit := c.Query("limit"); limit != "" {
			if limitInt, err := strconv.Atoi(limit); err != nil || limitInt < 0 || limitInt > 1000 {
				return c.Status(400).JSON(fiber.Map{
					"error":   "invalid_limit",
					"message": "Limit must be between 0 and 1000",
				})
			}
		}

		if offset := c.Query("offset"); offset != "" {
			if offsetInt, err := strconv.Atoi(offset); err != nil || offsetInt < 0 {
				return c.Status(400).JSON(fiber.Map{
					"error":   "invalid_offset",
					"message": "Offset must be a non-negative integer",
				})
			}
		}

		return c.Next()
	}
}

// isValidID validates ID format (numeric)
func isValidID(id string) bool {
	if len(id) == 0 || len(id) > 20 {
		return false
	}

	for _, char := range id {
		if char < '0' || char > '9' {
			return false
		}
	}

	return true
}

// ContentTypeValidationMiddleware validates content types for POST/PUT requests
func ContentTypeValidationMiddleware(allowedTypes ...string) fiber.Handler {
	if len(allowedTypes) == 0 {
		allowedTypes = []string{"application/json", "application/x-www-form-urlencoded", "multipart/form-data"}
	}

	return func(c *fiber.Ctx) error {
		method := c.Method()
		if method == "POST" || method == "PUT" || method == "PATCH" {
			contentType := c.Get("Content-Type")

			// Extract base content type (remove charset, boundary, etc.)
			if idx := strings.Index(contentType, ";"); idx > 0 {
				contentType = contentType[:idx]
			}
			contentType = strings.TrimSpace(contentType)

			// Check if content type is allowed
			allowed := slices.Contains(allowedTypes, contentType)

			if !allowed {
				return c.Status(415).JSON(fiber.Map{
					"error":   "unsupported_media_type",
					"message": fmt.Sprintf("Content-Type must be one of: %v", allowedTypes),
				})
			}
		}

		return c.Next()
	}
}
