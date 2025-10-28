package config

import (
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// getJWTSecret reads the JWT secret from the environment at call time
// so it reflects values loaded by godotenv in service mains.
func getJWTSecret() []byte {
	return []byte(os.Getenv("JWT_SECRET"))
}

// GenerateToken creates a signed JWT token for the given userID.
func GenerateToken(userID uint) (string, error) {
	now := time.Now()
	claims := jwt.MapClaims{
		"user_id":    userID,
		"token_type": "access",
		"exp":        now.Add(15 * time.Minute).Unix(),
		"iat":        now.Unix(),
		"iss":        "ts-timeline-system",
		"aud":        []string{"ts-timeline-users"},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(getJWTSecret())
}

// ValidateToken parses and validates a JWT token string.
//
// It ensures:
// - The signing method is HMAC (HS256).
// - The token is not expired and has a valid signature.
// - The "user_id" claim exists and can be converted to uint.
func ValidateToken(tokenStr string) (uint, error) {
	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrTokenSignatureInvalid
		}
		return getJWTSecret(), nil
	})
	if err != nil || !token.Valid {
		return 0, err
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return 0, jwt.ErrTokenInvalidClaims
	}
	if v, ok := claims["user_id"].(float64); ok {
		// Returns the userID encoded in the token
		return uint(v), nil
	}
	return 0, jwt.ErrTokenInvalidClaims
}
