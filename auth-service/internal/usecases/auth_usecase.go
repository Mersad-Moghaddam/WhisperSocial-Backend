package usecases

import (
	"log"
	"strings"

	"github.com/Mersad-Moghaddam/auth-service/internal/ports"
	"github.com/Mersad-Moghaddam/shared/config"
	"github.com/gofiber/fiber/v2"
	"golang.org/x/crypto/bcrypt"
)

type authUsecase struct {
	repo ports.UserRepository
}

func NewAuthUsecase(r ports.UserRepository) ports.AuthUsecase {
	return &authUsecase{
		repo: r,
	}
}

// Register handles user registration.
// Steps:
// 1. Hash the user's password using bcrypt.
// 2. Create a new user in the repository.
// 3. Return appropriate HTTP responses:
//   - 200 OK on success
//   - 409 Conflict if email already exists
//   - 500 Internal Server Error for other failures
func (uc *authUsecase) Register(c *fiber.Ctx, req ports.RegisterRequest) error {
	hashed, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		log.Println("cannot hash password:", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "internal error",
		})
	}
	user := &ports.User{
		Email:    req.Email,
		Password: string(hashed),
		Role:     "user",
		Status:   "active",
	}
	// Attempt to create the user in the repository
	if err := uc.repo.Create(user); err != nil {
		msg := strings.ToLower(err.Error())
		// Check if the error is related to a duplicate or unique constraint violation
		if strings.Contains(msg, "duplicate") || strings.Contains(msg, "unique") {
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{
				"error": "email already exists",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "user creation failed",
		})
	}
	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"message": "user registered",
	})
}

// Login handles user authentication.
// Steps:
// 1. Retrieve user by email.
// 2. Compare the provided password with the stored hash.
// 3. Generate JWT token on success.
// 4. Return appropriate HTTP responses:
//   - 200 OK with token
//   - 401 Unauthorized for invalid credentials
//   - 500 Internal Server Error if token creation fails
func (uc *authUsecase) Login(c *fiber.Ctx, req ports.LoginRequest) error {
	user, err := uc.repo.FindByEmail(req.Email)
	if err != nil || user == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "invalid credential",
		})
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "invalid credential",
		})
	}
	if user.Status == "deactivated" {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "account is deactivated",
		})
	}
	token, err := config.GenerateToken(user.ID, user.Email, user.Role)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "token creation failed",
		})
	}
	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"token": token,
	})
}
func (uc *authUsecase) UpdateInfo(c *fiber.Ctx, req ports.UpdateUserRequest) error {
	user, err := uc.repo.FindByEmail(req.OldEmail)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "user not found",
		})
	}

	if req.Password != "" {
		hashed, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "cannot hash password",
			})
		}
		user.Password = string(hashed)
	}

	if req.NewEmail != "" {
		user.Email = req.NewEmail
	}

	if err := uc.repo.Update(user); err != nil {
		msg := strings.ToLower(err.Error())
		if strings.Contains(msg, "duplicate") || strings.Contains(msg, "unique") {
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{
				"error": "email already exists",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "update failed",
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"message": "user updated",
	})
}
