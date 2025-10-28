package ports

import (
	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
)

type RegisterRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=6"`
}

type LoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}
type UpdateUserRequest struct {
	OldEmail string `json:"old_email" validate:"required,email"`
	NewEmail string `json:"new_email" validate:"omitempty,email"`
	Password string `json:"password" validate:"omitempty,min=6"`
}

func (req UpdateUserRequest) Validate() error {
	return validator.New().Struct(req)
}

func (req RegisterRequest) Validate() error {
	return validator.New().Struct(req)
}

func (req LoginRequest) Validate() error {
	return validator.New().Struct(req)
}

// defines the interface for interacting with user data storage.
type UserRepository interface {
	Create(user *User) error
	Update(user *User) error
	FindByEmail(email string) (*User, error)
}

// defines the business logic interface for authentication.
type AuthUsecase interface {
	Register(c *fiber.Ctx, req RegisterRequest) error
	Login(c *fiber.Ctx, req LoginRequest) error
	UpdateInfo(c *fiber.Ctx, req UpdateUserRequest) error
}
