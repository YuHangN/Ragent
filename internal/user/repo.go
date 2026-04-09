package user

import (
	"gorm.io/gorm"
)

// UserRepo 定义数据库操作接口，方便后续 mock 测试。
type UserRepo interface {
	FindByUsername(username string) (*User, error)
	FindByID(id int64) (*User, error)
	Create(u *User) error
	Update(u *User) error
	Delete(id int64) error
	Page(keyword string, page, size int) ([]User, int64, error)
	ExistsByUsername(username string, excludeID int64) (bool, error)
}

type gormUserRepo struct {
	db *gorm.DB
}

func (r *gormUserRepo) FindByUsername(username string) (*User, error) {
	var u User
	err := r.db.Where("username = ?", username).First(&u).Error
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *gormUserRepo) FindByID(id int64) (*User, error) {
	var u User
	err := r.db.First(&u, id).Error
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *gormUserRepo) Create(u *User) error {
	return r.db.Create(u).Error
}

func (r *gormUserRepo) Update(u *User) error {
	return r.db.Save(u).Error
}

func (r *gormUserRepo) Delete(id int64) error {
	return r.db.Delete(&User{}, id).Error
}

func (r *gormUserRepo) Page(keyword string, page, size int) ([]User, int64, error) {
	var users []User
	var total int64
	q := r.db.Model(&User{})
	if keyword != "" {
		q = q.Where("username LIKE ?", "%"+keyword+"%")
	}
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	offset := (page - 1) * size
	if err := q.Offset(offset).Limit(size).Order("update_time DESC").Find(&users).Error; err != nil {
		return nil, 0, err
	}
	return users, total, nil
}

func (r *gormUserRepo) ExistsByUsername(username string, excludeID int64) (bool, error) {
	var count int64
	q := r.db.Model(&User{}).Where("username = ?", username)
	if excludeID > 0 {
		q = q.Where("id != ?", excludeID)
	}
	err := q.Count(&count).Error
	return count > 0, err
}

// NewUserRepo 创建基于 GORM 的 UserRepo 实现。
func NewUserRepo(db *gorm.DB) UserRepo {
	return &gormUserRepo{db: db}
}
