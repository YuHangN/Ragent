package user

import (
	"net/http"
	"strconv"

	"github.com/YuHangN/ragent-go/pkg/apperror"
	"github.com/YuHangN/ragent-go/pkg/errorcode"
	"github.com/YuHangN/ragent-go/pkg/response"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	svc *UserService
}

func NewHandler(svc *UserService) *Handler {
	return &Handler{svc: svc}
}

// Login POST /auth/login
func (h *Handler) Login(c *gin.Context) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(apperror.NewClientMsg("请求参数错误"))
		return
	}
	vo, err := h.svc.Login(req.Username, req.Password)
	if err != nil {
		_ = c.Error(apperror.NewClientMsg(err.Error()))
		return
	}

	c.JSON(http.StatusOK, response.Success(vo))
}

// Logout POST /auth/logout  （JWT 无状态，客户端删除 token 即可，服务端直接返回成功）
func (h *Handler) Logout(c *gin.Context) {
	c.JSON(http.StatusOK, response.Success[any](nil))
}

// CurrentUser GET /user/me
func (h *Handler) CurrentUser(c *gin.Context) {
	c.JSON(http.StatusOK, response.Success(CurrentUserVO{
		UserID:   c.GetString("userID"),
		Username: c.GetString("username"),
		Role:     c.GetString("role"),
	}))
}

// PageUsers GET /users  (admin)
func (h *Handler) PageUsers(c *gin.Context) {
	keyword := c.Query("keyword")
	page, _ := strconv.Atoi(c.DefaultQuery("current", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	result, err := h.svc.PageQuery(keyword, page, size)
	if err != nil {
		_ = c.Error(apperror.NewServiceWrap("查询失败", err, errorcode.ServiceError))
		return
	}
	c.JSON(http.StatusOK, response.Success(result))
}

// CreateUser POST /users  (admin)
func (h *Handler) CreateUser(c *gin.Context) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Role     string `json:"role"`
		Avatar   string `json:"avatar"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(apperror.NewClientMsg("请求参数错误"))
		return
	}
	id, err := h.svc.Create(req.Username, req.Password, req.Role, req.Avatar)
	if err != nil {
		_ = c.Error(apperror.NewClientMsg(err.Error()))
		return
	}
	c.JSON(http.StatusOK, response.Success(id))
}

// UpdateUser PUT /users/:id  (admin)
func (h *Handler) UpdateUser(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		_ = c.Error(apperror.NewClientMsg("用户ID非法"))
		return
	}
	var req struct {
		Username *string `json:"username"`
		Password *string `json:"password"`
		Role     *string `json:"role"`
		Avatar   *string `json:"avatar"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(apperror.NewClientMsg("请求参数错误"))
		return
	}
	if err := h.svc.Update(id, req.Username, req.Password, req.Role, req.Avatar); err != nil {
		_ = c.Error(apperror.NewClientMsg(err.Error()))
		return
	}
	c.JSON(http.StatusOK, response.Success[any](nil))
}

// DeleteUser DELETE /users/:id  (admin)
func (h *Handler) DeleteUser(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		_ = c.Error(apperror.NewClientMsg("用户ID非法"))
		return
	}
	if err := h.svc.Delete(id); err != nil {
		_ = c.Error(apperror.NewClientMsg(err.Error()))
		return
	}
	c.JSON(http.StatusOK, response.Success[any](nil))
}

// ChangePassword PUT /user/password
func (h *Handler) ChangePassword(c *gin.Context) {
	userIDStr := c.GetString("userID")
	userID, err := strconv.ParseInt(userIDStr, 10, 64)
	if err != nil {
		_ = c.Error(apperror.NewClientMsg("用户ID非法"))
		return
	}

	var req struct {
		CurrentPassword string `json:"currentPassword"`
		NewPassword     string `json:"newPassword"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(apperror.NewClientMsg("请求参数错误"))
		return
	}
	if err := h.svc.ChangePassword(userID, req.CurrentPassword, req.NewPassword); err != nil {
		_ = c.Error(apperror.NewClientMsg(err.Error()))
		return
	}
	c.JSON(http.StatusOK, response.Success[any](nil))
}
