package ports

type CreatePostRequest struct {
	AuthorID uint   `json:"author_id"`
	Content  string `json:"content"`
}

type PostRepository interface {
	Save(post *Post) error
}

type EventPublisher interface {
	PublishPostCreated(post Post) error
}

type CreatePostUsecase interface {
	Create(req CreatePostRequest) (Post, error)
}
