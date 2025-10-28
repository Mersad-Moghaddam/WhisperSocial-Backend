package database

import (
	"github.com/Mersad-Moghaddam/post-service/internal/ports"
	"github.com/Mersad-Moghaddam/shared/config"
)

type postRepo struct{}

func NewPostRepository() ports.PostRepository {
	return &postRepo{}
}

func (r *postRepo) Save(post *ports.Post) error {
	return config.DB.Create(post).Error
}
