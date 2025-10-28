package database

import (
	"github.com/Mersad-Moghaddam/auth-service/internal/ports"
	"github.com/Mersad-Moghaddam/shared/config"
)

type userRepo struct{}

func NewUserRepository() ports.UserRepository {
	return &userRepo{}
}

func (r *userRepo) Create(user *ports.User) error {
	return config.DB.Create(user).Error
}

func (r *userRepo) Update(user *ports.User) error {
	oldUser, err := r.FindByEmail(user.Email)
	if err != nil {
		return err
	}
	return config.DB.Model(&ports.User{}).
		Where("email = ?", oldUser.Email).
		Updates(user).Error
}

func (r *userRepo) FindByEmail(email string) (*ports.User, error) {
	var user ports.User
	if err := config.DB.Where("email = ?", email).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}
