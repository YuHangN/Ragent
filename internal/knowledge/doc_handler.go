package knowledge

import (
	"io"
	"net/http"

	"github.com/YuHangN/ragent-go/pkg/apperror"
	"github.com/YuHangN/ragent-go/pkg/errorcode"
	"github.com/YuHangN/ragent-go/pkg/response"
	"github.com/YuHangN/ragent-go/pkg/usercontext"
	"github.com/gin-gonic/gin"
)

// DocHandler 对应 Java KnowledgeDocumentController。
type DocHandler struct {
	svc      *DocService
	chunkLog *ChunkLogService
}

func NewDocHandler(svc *DocService, chunkLog *ChunkLogService) *DocHandler {
	return &DocHandler{svc: svc, chunkLog: chunkLog}
}

// UploadDoc POST /knowledge-base/:kb-id/docs/upload
func (h *DocHandler) UploadDoc(c *gin.Context) {
	kbID := c.Param("kb-id")

	var req DocUploadRequest
	if err := c.ShouldBind(&req); err != nil {
		_ = c.Error(apperror.NewClientMsg("请求参数错误"))
		return
	}

	var (
		reader   io.Reader
		fileName string
		fileSize int64
	)
	fileHeader, err := c.FormFile("file")

	if err == nil && fileHeader != nil {
		f, err := fileHeader.Open()
		if err != nil {
			_ = c.Error(apperror.NewClientMsg("文件读取失败"))
			return
		}
		defer f.Close()
		reader = f
		fileName = fileHeader.Filename
		fileSize = fileHeader.Size
	} else {
		// URL 来源，用 sourceLocation 作为文件名
		fileName = req.SourceLocation
	}

	vo, err := h.svc.Upload(
		kbID,
		req.SourceType, req.SourceLocation,
		req.ProcessMode, req.ScheduleCron,
		req.ChunkStrategy, req.ChunkConfig, req.TargetPartition,
		req.ScheduleEnabled,
		reader, fileName, fileSize,
		usercontext.Username(c),
	)
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response.Success(vo))
}

// UpdateDoc PUT /knowledge-base/docs/:doc-id
// 对应 Java KnowledgeDocumentController.update。
func (h *DocHandler) UpdateDoc(c *gin.Context) {
	docID := c.Param("doc-id")
	var req DocUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(apperror.NewClientMsg("请求参数错误"))
		return
	}
	if err := h.svc.Update(docID, req, usercontext.Username(c)); err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response.Success[any](nil))
}

// StartChunk POST /knowledge-base/docs/:doc-id/chunk
func (h *DocHandler) StartChunk(c *gin.Context) {
	if err := h.svc.StartChunk(c.Param("doc-id")); err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response.Success[any](nil))
}

// DeleteDoc DELETE /knowledge-base/docs/:doc-id
func (h *DocHandler) DeleteDoc(c *gin.Context) {
	if err := h.svc.Delete(c.Param("doc-id")); err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response.Success[any](nil))
}

// GetDoc GET /knowledge-base/docs/:doc-id
func (h *DocHandler) GetDoc(c *gin.Context) {
	vo, err := h.svc.Get(c.Param("doc-id"))
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response.Success(vo))
}

// PageDocs GET /knowledge-base/:kb-id/docs
func (h *DocHandler) PageDocs(c *gin.Context) {
	kbID := c.Param("kb-id")
	var req DocPageRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		_ = c.Error(apperror.NewClientMsg("请求参数错误"))
		return
	}
	result, err := h.svc.Page(kbID, req.Status, req.Keyword, req.PageNo, req.PageSize)
	if err != nil {
		_ = c.Error(apperror.NewServiceWrap("查询失败", err, errorcode.ServiceError))
		return
	}
	c.JSON(http.StatusOK, response.Success(result))
}

// SearchDocs GET /knowledge-base/docs/search
// 对应 Java KnowledgeDocumentController.search，之前 Phase 3 的 routes.go 漏挂的路由。
func (h *DocHandler) SearchDocs(c *gin.Context) {
	var req DocSearchRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		_ = c.Error(apperror.NewClientMsg("请求参数错误"))
		return
	}
	result, err := h.svc.Search(req.Keyword, req.Limit)
	if err != nil {
		_ = c.Error(apperror.NewServiceWrap("搜索失败", err, errorcode.ServiceError))
		return
	}
	c.JSON(http.StatusOK, response.Success(result))
}

// EnableDoc PATCH /knowledge-base/docs/:doc-id/enable
func (h *DocHandler) EnableDoc(c *gin.Context) {
	enabled := c.Query("value") == "true"
	if err := h.svc.Enable(c.Param("doc-id"), enabled, usercontext.Username(c)); err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response.Success[any](nil))
}

func (h *DocHandler) GetChunkLogs(c *gin.Context) {
	docID := c.Param("doc-id")
	var req ChunkLogPageRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		_ = c.Error(apperror.NewClientMsg("请求参数错误"))
		return
	}

	result, err := h.chunkLog.Page(docID, req.PageNo, req.PageSize)
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response.Success(result))
}
