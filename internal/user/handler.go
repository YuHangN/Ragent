package user

import (
	"net/http"
	"strconv"

	"github.com/YuHangN/ragent-go/pkg/apperror"
	"github.com/YuHangN/ragent-go/pkg/response"
	"github.com/YuHangN/ragent-go/pkg/usercontext"
	"github.com/gin-gonic/gin"
)

// AuthHandler 负责登录登出，对应 Java AuthController。
type AuthHandler struct {
	svc *AuthService
}

func NewAuthHandler(svc *AuthService) *AuthHandler {
	return &AuthHandler{svc: svc}
}

// Login POST /auth/login
func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(apperror.NewClientMsg("请求参数错误"))
		return
	}

	vo, err := h.svc.Login(req)
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response.Success(vo))
}

// Logout POST /auth/logout
func (h *AuthHandler) Logout(c *gin.Context) {
	_ = h.svc.Logout()
	c.JSON(http.StatusOK, response.Success[any](nil))
}

// UserHandler 负责用户 CRUD / 改密
type UserHandler struct {
	svc *UserService
}

func NewUserHandler(svc *UserService) *UserHandler {
	return &UserHandler{svc: svc}
}

// CurrentUser GET /user/me
// 从 usercontext 读取完整 LoginUser（含 avatar）
func (h *UserHandler) CurrentUser(c *gin.Context) {
	u := usercontext.Require(c)
	c.JSON(http.StatusOK, response.Success(CurrentUserVO{
		UserID:   u.UserID,
		Username: u.Username,
		Role:     u.Role,
		Avatar:   u.Avatar,
	}))
}

// PageUsers GET /users  (admin)
func (h *UserHandler) PageUsers(c *gin.Context) {
	var req UserPageRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		_ = c.Error(apperror.NewClientMsg("请求参数错误"))
		return
	}
	result, err := h.svc.PageQuery(req)
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response.Success(result))
}

// CreateUser POST /users  (admin)
func (h *UserHandler) CreateUser(c *gin.Context) {
	var req UserCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(apperror.NewClientMsg("请求参数错误"))
		return
	}
	id, err := h.svc.Create(req)
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response.Success(id))
}

// UpdateUser PUT /users/:id  (admin)
func (h *UserHandler) UpdateUser(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		_ = c.Error(apperror.NewClientMsg("用户ID非法"))
		return
	}
	var req UserUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(apperror.NewClientMsg("请求参数错误"))
		return
	}
	if err := h.svc.Update(id, req); err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response.Success[any](nil))
}

// DeleteUser DELETE /users/:id  (admin)
func (h *UserHandler) DeleteUser(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		_ = c.Error(apperror.NewClientMsg("用户ID非法"))
		return
	}
	if err := h.svc.Delete(id); err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response.Success[any](nil))
}

// ChangePassword PUT /user/password
func (h *UserHandler) ChangePassword(c *gin.Context) {
	u := usercontext.Require(c)
	userID, err := strconv.ParseInt(u.UserID, 10, 64)
	if err != nil {
		_ = c.Error(apperror.NewClientMsg("用户ID非法"))
		return
	}
	var req ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(apperror.NewClientMsg("请求参数错误"))
		return
	}
	if err := h.svc.ChangePassword(userID, req); err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response.Success[any](nil))
}
