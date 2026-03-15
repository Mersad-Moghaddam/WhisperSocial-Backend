package database

import (
	"github.com/Mersad-Moghaddam/admin-service/internal/ports"
	"github.com/Mersad-Moghaddam/shared/config"
)

type adminRepo struct{}

func NewAdminRepository() ports.AdminRepository { return &adminRepo{} }

func (r *adminRepo) ListUsers(filter ports.UserFilters) ([]ports.User, error) {
	var users []ports.User
	q := config.DB.Model(&ports.User{})
	if filter.Status != "" {
		q = q.Where("status = ?", filter.Status)
	}
	if filter.CreatedAfter != nil {
		q = q.Where("created_at >= ?", *filter.CreatedAfter)
	}
	if filter.CreatedBefore != nil {
		q = q.Where("created_at <= ?", *filter.CreatedBefore)
	}
	err := q.Order("id DESC").Limit(filter.Limit).Offset(filter.Offset).Find(&users).Error
	return users, err
}
func (r *adminRepo) GetUserByID(id uint) (*ports.User, error) {
	var user ports.User
	if err := config.DB.First(&user, id).Error; err != nil {
		return nil, err
	}
	return &user, nil
}
func (r *adminRepo) UpdateUserStatus(id uint, status string) error {
	return config.DB.Model(&ports.User{}).Where("id = ?", id).Update("status", status).Error
}
func (r *adminRepo) ListPosts(filter ports.PostFilters) ([]ports.Post, error) {
	var posts []ports.Post
	q := config.DB.Model(&ports.Post{}).Preload("Author")
	if filter.UserID > 0 {
		q = q.Where("author_id = ?", filter.UserID)
	}
	if filter.StartDate != nil {
		q = q.Where("created_at >= ?", *filter.StartDate)
	}
	if filter.EndDate != nil {
		q = q.Where("created_at <= ?", *filter.EndDate)
	}
	err := q.Order("id DESC").Limit(filter.Limit).Offset(filter.Offset).Find(&posts).Error
	return posts, err
}
func (r *adminRepo) GetPostByID(id uint) (*ports.Post, error) {
	var post ports.Post
	if err := config.DB.Preload("Author").First(&post, id).Error; err != nil {
		return nil, err
	}
	return &post, nil
}
func (r *adminRepo) DeletePost(id uint) error { return config.DB.Delete(&ports.Post{}, id).Error }
func (r *adminRepo) CountStats() (map[string]int64, error) {
	out := map[string]int64{}
	var totalUsers, totalPosts, activeUsers int64
	if err := config.DB.Table("users").Count(&totalUsers).Error; err != nil {
		return nil, err
	}
	if err := config.DB.Table("posts").Where("deleted_at IS NULL").Count(&totalPosts).Error; err != nil {
		return nil, err
	}
	if err := config.DB.Table("users").Where("status = ?", "active").Count(&activeUsers).Error; err != nil {
		return nil, err
	}
	out["total_users"] = totalUsers
	out["total_posts"] = totalPosts
	out["active_users"] = activeUsers
	return out, nil
}
func (r *adminRepo) ListFollowerIDs(authorID uint) ([]uint, error) {
	var rows []struct{ FollowerID uint }
	if err := config.DB.Table("followers").Select("follower_id").Where("user_id = ?", authorID).Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]uint, 0, len(rows))
	for _, v := range rows {
		out = append(out, v.FollowerID)
	}
	return out, nil
}
