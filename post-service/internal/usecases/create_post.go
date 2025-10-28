package usecases

import (
	"time"

	"github.com/Mersad-Moghaddam/post-service/internal/ports"
)

type createPostUsecase struct {
	repo      ports.PostRepository
	publisher ports.EventPublisher
}

func NewCreatedPostUsecase(repo ports.PostRepository, publisher ports.EventPublisher) ports.CreatePostUsecase {
	return &createPostUsecase{
		repo:      repo,
		publisher: publisher,
	}
}

// Create handles the creation of a new post.
// Steps:
// 1. Constructs a new Post object with the provided content and author ID.
// 2. Sets the creation timestamp to the current UTC time.
// 3. Saves the post using the repository.
// 4. Publishes a "post created" event to notify other services (e.g., fan-out).
// 5. Returns the created Post object or an error if any step fails.
func (uc createPostUsecase) Create(req ports.CreatePostRequest) (ports.Post, error) {
	post := ports.Post{
		AuthorID:  req.AuthorID,
		Content:   req.Content,
		CreatedAt: time.Now().UTC(),
	}

	if err := uc.repo.Save(&post); err != nil {
		return ports.Post{}, err
	}

	if err := uc.publisher.PublishPostCreated(post); err != nil {
		return ports.Post{}, err
	}

	return post, nil
}
