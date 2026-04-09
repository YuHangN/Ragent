package user

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/YuHangN/ragent-go/pkg/jwt"
	"golang.org/x/crypto/bcrypt"
)

const defaultAvatarURL = "https://avatars.githubusercontent.com/u/583231?v=4"

// UserService 封装所有用户相关业务逻辑。
type UserService struct {
	repo         UserRepo
	jwtSecret    string
	jwtExpireHrs int
}

// NewUserService 创建 UserService 实例。
func NewUserService(repo UserRepo, jwtSecret string, jwtExpireHrs int) *UserService {
	return &UserService{repo: repo, jwtSecret: jwtSecret, jwtExpireHrs: jwtExpireHrs}
}

// Login 验证用户名密码，返回 LoginVO（含 JWT token）。
func (s *UserService) Login(username, password string) (*LoginVO, error) {
	if strings.TrimSpace(username) == "" || strings.TrimSpace(password) == "" {
		return nil, errors.New("用户名或密码不能为空")
	}

	u, err := s.repo.FindByUsername(username)
	if err != nil || u == nil {
		return nil, errors.New("用户名或密码错误")
	}
	if !CheckPassword(password, u.Password) {
		return nil, errors.New("用户名或密码错误")
	}
	token, err := jwt.Sign(
		jwt.UserClaims{UserID: idStr(u.ID), Username: u.Username, Role: u.Role},
		s.jwtSecret,
		time.Duration(s.jwtExpireHrs)*time.Hour,
	)
	if err != nil {
		return nil, errors.New("生成 token 失败")
	}
	avatar := u.Avatar
	if avatar == "" {
		avatar = defaultAvatarURL
	}

	return &LoginVO{UserID: idStr(u.ID), Role: u.Role, Token: token, Avatar: avatar}, nil
}

// GetByID 通过 ID 查询用户，供鉴权中间件使用。
func (s *UserService) GetByID(id int64) (*User, error) {
	return s.repo.FindByID(id)
}

// PageQuery 分页查询用户（管理员）
func (s *UserService) PageQuery(keyword string, page, size int) (*PageResult[UserVO], error) {
	if page <= 0 {
		page = 1
	}
	if size <= 0 || size > 100 {
		size = 20
	}
	users, total, err := s.repo.Page(keyword, page, size)
	if err != nil {
		return nil, err
	}

	records := make([]UserVO, 0, len(users))
	for _, u := range users {
		records = append(records, toVO(u))
	}

	return &PageResult[UserVO]{Total: total, Records: records}, nil
}

// Create 新建用户（管理员）。
func (s *UserService) Create(username, password, role, avatar string) (string, error) {
	if strings.TrimSpace(username) == "" {
		return "", errors.New("用户名不能为空")
	}
	if strings.TrimSpace(password) == "" {
		return "", errors.New("密码不能为空")
	}
	if strings.EqualFold(username, "admin") {
		return "", errors.New("默认管理员用户名不可用")
	}

	normalized, err := NormalizeRole(role)
	if err != nil {
		return "", err
	}
	exists, err := s.repo.ExistsByUsername(username, 0)
	if err != nil {
		return "", err
	}
	if exists {
		return "", errors.New("用户名已存在")
	}

	hashed, err := HashPassword(password)
	if err != nil {
		return "", errors.New("密码加密失败")
	}
	u := &User{Username: username, Password: hashed, Role: normalized, Avatar: avatar}
	if err := s.repo.Create(u); err != nil {
		return "", err
	}

	return idStr(u.ID), nil
}

// Update 更新用户信息（管理员）。
func (s *UserService) Update(id int64, username, password, role, avatar *string) error {
	u, err := s.repo.FindByID(id)
	if err != nil {
		return errors.New("用户不存在")
	}
	if strings.EqualFold(u.Username, "admin") {
		return errors.New("默认管理员不允许修改")
	}

	if username != nil {
		if strings.TrimSpace(*username) == "" {
			return errors.New("用户名不能为空")
		}
		if strings.EqualFold(*username, "admin") {
			return errors.New("默认管理员用户名不可用")
		}
		exists, _ := s.repo.ExistsByUsername(*username, id)
		if exists {
			return errors.New("用户名已存在")
		}
		u.Username = *username
	}

	if password != nil {
		if strings.TrimSpace(*password) == "" {
			return errors.New("新密码不能为空")
		}
		hashed, err := HashPassword(*password)
		if err != nil {
			return errors.New("密码加密失败")
		}
		u.Password = hashed
	}

	if role != nil {
		normalized, err := NormalizeRole(*role)
		if err != nil {
			return err
		}
		u.Role = normalized
	}

	if avatar != nil {
		u.Avatar = *avatar
	}

	return s.repo.Update(u)
}

// Delete 删除用户（管理员）。
func (s *UserService) Delete(id int64) error {
	u, err := s.repo.FindByID(id)
	if err != nil {
		return errors.New("用户不存在")
	}
	if strings.EqualFold(u.Username, "admin") {
		return errors.New("默认管理员不允许删除")
	}
	return s.repo.Delete(id)
}

// ChangePassword 修改当前用户密码。
func (s *UserService) ChangePassword(userID int64, currentPwd, newPwd string) error {
	if strings.TrimSpace(currentPwd) == "" || strings.TrimSpace(newPwd) == "" {
		return errors.New("密码不能为空")
	}
	u, err := s.repo.FindByID(userID)
	if err != nil {
		return errors.New("用户不存在")
	}
	if !CheckPassword(currentPwd, u.Password) {
		return errors.New("当前密码不正确")
	}
	hashed, err := HashPassword(newPwd)
	if err != nil {
		return errors.New("密码加密失败")
	}
	u.Password = hashed
	return s.repo.Update(u)
}

// NormalizeRole 校验并规范化角色字符串。
func NormalizeRole(role string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "admin":
		return "admin", nil
	case "user", "":
		return "user", nil
	default:
		return "", errors.New("角色类型不合法")
	}
}

// HashPassword 用 bcrypt 对密码加密。
func HashPassword(plain string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// CheckPassword 验证明文密码与 bcrypt hash 是否匹配。
func CheckPassword(plain, hashed string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hashed), []byte(plain)) == nil
}

func idStr(id int64) string {
	return fmt.Sprintf("%d", id)
}

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
