package usecases

import (
	"fmt"

	"github.com/Mersad-Moghaddam/admin-service/internal/ports"
	"github.com/Mersad-Moghaddam/shared/config"
)

type adminUC struct{ repo ports.AdminRepository }

func NewAdminUsecase(r ports.AdminRepository) ports.AdminUsecase { return &adminUC{repo: r} }

func (u *adminUC) ListUsers(filter ports.UserFilters) ([]ports.User, error) {
	return u.repo.ListUsers(filter)
}
func (u *adminUC) GetUserByID(id uint) (*ports.User, error) { return u.repo.GetUserByID(id) }
func (u *adminUC) UpdateUserStatus(id uint, status string) error {
	return u.repo.UpdateUserStatus(id, status)
}
func (u *adminUC) ListPosts(filter ports.PostFilters) ([]ports.Post, error) {
	return u.repo.ListPosts(filter)
}
func (u *adminUC) GetPostByID(id uint) (*ports.Post, error) { return u.repo.GetPostByID(id) }
func (u *adminUC) Stats() (map[string]int64, error)         { return u.repo.CountStats() }

func (u *adminUC) DeletePost(id uint) error {
	post, err := u.repo.GetPostByID(id)
	if err != nil {
		return err
	}
	if err := u.repo.DeletePost(id); err != nil {
		return err
	}
	followerIDs, err := u.repo.ListFollowerIDs(post.AuthorID)
	if err != nil {
		return err
	}
	for _, followerID := range followerIDs {
		key := fmt.Sprintf("timeline:%d", followerID)
		_ = config.RedisClient.ZRem(config.Ctx, key, fmt.Sprintf("%d", post.ID)).Err()
	}
	return config.RedisClient.ZRem(config.Ctx, fmt.Sprintf("timeline:%d", post.AuthorID), fmt.Sprintf("%d", post.ID)).Err()
}
