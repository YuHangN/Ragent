package user

import (
	"strings"
	"time"

	"github.com/YuHangN/ragent-go/pkg/apperror"
	"github.com/YuHangN/ragent-go/pkg/jwt"
)

const defaultAvatarURL = "https://avatars.githubusercontent.com/u/583231?v=4"

// AuthService 负责登录/登出，对齐 Java AuthService。
type AuthService struct {
	repo         UserRepo
	jwtSecret    string
	jwtExpireHrs int
}

// NewAuthService 构造 AuthService。
func NewAuthService(repo UserRepo, jwtSecret string, jwtExpireHrs int) *AuthService {
	return &AuthService{repo: repo, jwtSecret: jwtSecret, jwtExpireHrs: jwtExpireHrs}
}

// Login 校验用户名密码，签发 JWT，返回 LoginVO。
func (s *AuthService) Login(req LoginRequest) (*LoginVO, error) {
	// 1. 校验用户名密码
	username := strings.TrimSpace(req.Username)
	password := strings.TrimSpace(req.Password)
	if username == "" || password == "" {
		return nil, apperror.NewClientMsg("用户名或密码不能为空")
	}
	u, err := s.repo.FindByUsername(username)
	if err != nil || u == nil {
		return nil, apperror.NewClientMsg("用户名或密码错误")
	}
	if !CheckPassword(password, u.Password) {
		return nil, apperror.NewClientMsg("用户名或密码错误")
	}
	avatar := u.Avatar
	if avatar == "" {
		avatar = defaultAvatarURL
	}

	// 2. 签发 JWT，包含用户 ID、用户名、角色、头像等信息。
	token, err := jwt.Sign(
		jwt.UserClaims{
			UserID:   idStr(u.ID),
			Username: u.Username,
			Role:     u.Role,
			Avatar:   avatar,
		},
		s.jwtSecret,
		time.Duration(s.jwtExpireHrs)*time.Hour,
	)
	if err != nil {
		return nil, apperror.NewServiceMsg("生成 token 失败")
	}

	return &LoginVO{
		UserID: idStr(u.ID),
		Role:   u.Role,
		Token:  token,
		Avatar: avatar,
	}, nil
}

// Logout 是空操作（JWT 无状态）。
// 对齐 Java AuthService.logout() 的签名，为将来接入 token 黑名单预留。
func (s *AuthService) Logout() error {
	return nil
}
