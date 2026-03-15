package usecases

import (
	"errors"
	"time"

	"github.com/Mersad-Moghaddam/post-service/internal/ports"
	"github.com/Mersad-Moghaddam/shared/config"
)

type createPostUsecase struct {
	repo      ports.PostRepository
	publisher ports.EventPublisher
}

type userRecord struct {
	ID     uint
	Status string
}

var ErrUserCannotPost = errors.New("user cannot create posts")

func NewCreatedPostUsecase(repo ports.PostRepository, publisher ports.EventPublisher) ports.CreatePostUsecase {
	return &createPostUsecase{repo: repo, publisher: publisher}
}

func (uc createPostUsecase) Create(req ports.CreatePostRequest) (ports.Post, error) {
	var user userRecord
	if err := config.DB.Table("users").Select("id,status").Where("id = ?", req.AuthorID).First(&user).Error; err != nil {
		return ports.Post{}, err
	}
	if user.Status == "deactivated" || user.Status == "restricted" {
		return ports.Post{}, ErrUserCannotPost
	}

	post := ports.Post{AuthorID: req.AuthorID, Content: req.Content, CreatedAt: time.Now().UTC()}
	if err := uc.repo.Save(&post); err != nil {
		return ports.Post{}, err
	}
	if err := uc.publisher.PublishPostCreated(post); err != nil {
		return ports.Post{}, err
	}

	return post, nil
}
