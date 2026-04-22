package user

import (
	"fmt"
	"strings"

	"github.com/YuHangN/ragent-go/pkg/apperror"
	"golang.org/x/crypto/bcrypt"
)

const defaultAdminUsername = "admin"

// UserService 负责用户 CRUD 和改密码
type UserService struct {
	repo UserRepo
}

// NewUserService 构造 UserService。
func NewUserService(repo UserRepo) *UserService {
	return &UserService{repo: repo}
}

// GetByID 通过 ID 查询用户。
func (s *UserService) GetByID(id int64) (*User, error) {
	return s.repo.FindByID(id)
}

// PageQuery 分页查询用户（管理员用）。
func (s *UserService) PageQuery(req UserPageRequest) (*PageResult[UserVO], error) {
	page := req.Current
	if page <= 0 {
		page = 1
	}
	size := req.Size
	if size <= 0 || size > 100 {
		size = 20
	}
	users, total, err := s.repo.Page(req.Keyword, page, size)
	if err != nil {
		return nil, apperror.NewServiceWrap("查询失败", err, nil)
	}
	records := make([]UserVO, 0, len(users))
	for _, u := range users {
		records = append(records, toVO(u))
	}
	return &PageResult[UserVO]{Total: total, Records: records}, nil
}

// Create 新建用户（管理员）。
func (s *UserService) Create(req UserCreateRequest) (string, error) {
	username := strings.TrimSpace(req.Username)
	password := strings.TrimSpace(req.Password)
	if username == "" {
		return "", apperror.NewClientMsg("用户名不能为空")
	}
	if password == "" {
		return "", apperror.NewClientMsg("密码不能为空")
	}
	if strings.EqualFold(username, defaultAdminUsername) {
		return "", apperror.NewClientMsg("默认管理员用户名不可用")
	}
	role, err := NormalizeRole(req.Role)
	if err != nil {
		return "", err
	}
	exists, err := s.repo.ExistsByUsername(username, 0)
	if err != nil {
		return "", apperror.NewServiceWrap("用户名唯一性检查失败", err, nil)
	}
	if exists {
		return "", apperror.NewClientMsg("用户名已存在")
	}
	hashed, err := HashPassword(password)
	if err != nil {
		return "", apperror.NewServiceMsg("密码加密失败")
	}
	u := &User{Username: username, Password: hashed, Role: role.String(), Avatar: req.Avatar}
	if err := s.repo.Create(u); err != nil {
		return "", apperror.NewServiceWrap("创建用户失败", err, nil)
	}
	return idStr(u.ID), nil
}

// Update 更新用户（管理员）。
func (s *UserService) Update(id int64, req UserUpdateRequest) error {
	u, err := s.repo.FindByID(id)
	if err != nil || u == nil {
		return apperror.NewClientMsg("用户不存在")
	}
	if strings.EqualFold(u.Username, defaultAdminUsername) {
		return apperror.NewClientMsg("默认管理员不允许修改")
	}
	if req.Username != nil {
		uname := strings.TrimSpace(*req.Username)
		if uname == "" {
			return apperror.NewClientMsg("用户名不能为空")
		}
		if strings.EqualFold(uname, defaultAdminUsername) {
			return apperror.NewClientMsg("默认管理员用户名不可用")
		}
		exists, err := s.repo.ExistsByUsername(uname, id)
		if err != nil {
			return apperror.NewServiceWrap("用户名唯一性检查失败", err, nil)
		}
		if exists {
			return apperror.NewClientMsg("用户名已存在")
		}
		u.Username = uname
	}
	if req.Password != nil {
		pwd := strings.TrimSpace(*req.Password)
		if pwd == "" {
			return apperror.NewClientMsg("新密码不能为空")
		}
		hashed, err := HashPassword(pwd)
		if err != nil {
			return apperror.NewServiceMsg("密码加密失败")
		}
		u.Password = hashed
	}
	if req.Role != nil {
		role, err := NormalizeRole(*req.Role)
		if err != nil {
			return err
		}
		u.Role = role.String()
	}
	if req.Avatar != nil {
		u.Avatar = *req.Avatar
	}
	if err := s.repo.Update(u); err != nil {
		return apperror.NewServiceWrap("更新用户失败", err, nil)
	}
	return nil
}

// Delete 删除用户（管理员）。
func (s *UserService) Delete(id int64) error {
	u, err := s.repo.FindByID(id)
	if err != nil || u == nil {
		return apperror.NewClientMsg("用户不存在")
	}
	if strings.EqualFold(u.Username, defaultAdminUsername) {
		return apperror.NewClientMsg("默认管理员不允许删除")
	}
	if err := s.repo.Delete(id); err != nil {
		return apperror.NewServiceWrap("删除用户失败", err, nil)
	}
	return nil
}

// ChangePassword 修改当前用户密码。
func (s *UserService) ChangePassword(userID int64, req ChangePasswordRequest) error {
	current := strings.TrimSpace(req.CurrentPassword)
	next := strings.TrimSpace(req.NewPassword)
	if current == "" || next == "" {
		return apperror.NewClientMsg("密码不能为空")
	}
	u, err := s.repo.FindByID(userID)
	if err != nil || u == nil {
		return apperror.NewClientMsg("用户不存在")
	}
	if !CheckPassword(current, u.Password) {
		return apperror.NewClientMsg("当前密码不正确")
	}
	hashed, err := HashPassword(next)
	if err != nil {
		return apperror.NewServiceMsg("密码加密失败")
	}
	u.Password = hashed
	if err := s.repo.Update(u); err != nil {
		return apperror.NewServiceWrap("更新密码失败", err, nil)
	}
	return nil
}

// HashPassword 用 bcrypt 加密。
func HashPassword(plain string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// CheckPassword 验证明文与 hash 是否匹配。
func CheckPassword(plain, hashed string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hashed), []byte(plain)) == nil
}

func idStr(id int64) string { return fmt.Sprintf("%d", id) }

func toVO(u User) UserVO {
	return UserVO{
		ID:        idStr(u.ID),
		Username:  u.Username,
		Role:      u.Role,
		Avatar:    u.Avatar,
		CreatedAt: u.CreatedAt,
		UpdatedAt: u.UpdatedAt,
	}
}
