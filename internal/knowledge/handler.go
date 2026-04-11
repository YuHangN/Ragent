package knowledge

import (
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/YuHangN/ragent-go/pkg/errorcode"
	"github.com/YuHangN/ragent-go/pkg/response"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	kb    *KBService
	doc   *DocService
	chunk *ChunkService
}

func NewHandler(kb *KBService, doc *DocService, chunk *ChunkService) *Handler {
	return &Handler{kb: kb, doc: doc, chunk: chunk}
}

// ──────────────────────── KB ────────────────────────

func (h *Handler) CreateKB(c *gin.Context) {
	var req struct {
		Name           string `json:"name"`
		EmbeddingModel string `json:"embeddingModel"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.Fail[any](errorcode.ClientError, "请求参数错误"))
		return
	}
	id, err := h.kb.Create(req.Name, req.EmbeddingModel, c.GetString("username"))
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Fail[any](errorcode.ClientError, err.Error()))
		return
	}
	c.JSON(http.StatusOK, response.Success(id))
}

func (h *Handler) RenameKB(c *gin.Context) {
	kbID, err := strconv.ParseInt(c.Param("kb-id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Fail[any](errorcode.ClientError, "知识库ID非法"))
		return
	}
	var req struct {
		Name string `json:"name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.Fail[any](errorcode.ClientError, "请求参数错误"))
		return
	}
	if err := h.kb.Rename(kbID, req.Name, c.GetString("username")); err != nil {
		c.JSON(http.StatusBadRequest, response.Fail[any](errorcode.ClientError, err.Error()))
		return
	}
	c.JSON(http.StatusOK, response.Success[any](nil))
}

func (h *Handler) DeleteKB(c *gin.Context) {
	kbID, err := strconv.ParseInt(c.Param("kb-id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Fail[any](errorcode.ClientError, "知识库ID非法"))
		return
	}
	if err := h.kb.Delete(kbID); err != nil {
		c.JSON(http.StatusBadRequest, response.Fail[any](errorcode.ClientError, err.Error()))
		return
	}
	c.JSON(http.StatusOK, response.Success[any](nil))
}

func (h *Handler) GetKB(c *gin.Context) {
	kbID, err := strconv.ParseInt(c.Param("kb-id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Fail[any](errorcode.ClientError, "知识库ID非法"))
		return
	}
	vo, err := h.kb.GetByID(kbID)
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Fail[any](errorcode.ClientError, err.Error()))
		return
	}
	c.JSON(http.StatusOK, response.Success(vo))
}

func (h *Handler) PageKB(c *gin.Context) {
	name := c.Query("name")
	page, _ := strconv.Atoi(c.DefaultQuery("current", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	result, err := h.kb.Page(name, page, size)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.Fail[any](errorcode.ServiceError, "查询失败"))
		return
	}
	c.JSON(http.StatusOK, response.Success(result))
}

// ──────────────────────── Document ────────────────────────

func (h *Handler) UploadDoc(c *gin.Context) {
	kbID := c.Param("kb-id")
	sourceType := c.PostForm("sourceType")
	sourceLocation := c.PostForm("sourceLocation")
	processMode := c.PostForm("processMode")
	scheduleCron := c.PostForm("scheduleCron")
	scheduleEnabled := c.PostForm("scheduleEnabled") == "true"

	var (
		reader   io.Reader
		fileName string
		fileSize int64
	)
	fileHeader, err := c.FormFile("file")
	if err == nil && fileHeader != nil {
		f, err := fileHeader.Open()
		if err != nil {
			c.JSON(http.StatusBadRequest, response.Fail[any](errorcode.ClientError, "文件读取失败"))
			return
		}
		defer f.Close()
		reader = f
		fileName = fileHeader.Filename
		fileSize = fileHeader.Size
	} else {
		// URL 来源，用 sourceLocation 作为文件名
		fileName = sourceLocation
	}

	vo, err := h.doc.Upload(kbID, sourceType, sourceLocation, processMode, scheduleCron,
		scheduleEnabled, reader, fileName, fileSize, c.GetString("username"))
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Fail[any](errorcode.ClientError, err.Error()))
		return
	}
	c.JSON(http.StatusOK, response.Success(vo))
}

func (h *Handler) StartChunk(c *gin.Context) {
	if err := h.doc.StartChunk(c.Param("doc-id")); err != nil {
		c.JSON(http.StatusBadRequest, response.Fail[any](errorcode.ClientError, err.Error()))
		return
	}
	c.JSON(http.StatusOK, response.Success[any](nil))
}

func (h *Handler) DeleteDoc(c *gin.Context) {
	if err := h.doc.Delete(c.Param("doc-id")); err != nil {
		c.JSON(http.StatusBadRequest, response.Fail[any](errorcode.ClientError, err.Error()))
		return
	}
	c.JSON(http.StatusOK, response.Success[any](nil))
}

func (h *Handler) GetDoc(c *gin.Context) {
	vo, err := h.doc.Get(c.Param("docId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Fail[any](errorcode.ClientError, err.Error()))
		return
	}
	c.JSON(http.StatusOK, response.Success(vo))
}

func (h *Handler) PageDocs(c *gin.Context) {
	kbID := c.Param("kb-id")
	status := c.Query("status")
	keyword := c.Query("keyword")
	page, _ := strconv.Atoi(c.DefaultQuery("pageNo", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("pageSize", "10"))
	result, err := h.doc.Page(kbID, status, keyword, page, size)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.Fail[any](errorcode.ServiceError, "查询失败"))
		return
	}
	c.JSON(http.StatusOK, response.Success(result))
}

func (h *Handler) EnableDoc(c *gin.Context) {
	enabled := c.Query("value") == "true"
	if err := h.doc.Enable(c.Param("docId"), enabled); err != nil {
		c.JSON(http.StatusBadRequest, response.Fail[any](errorcode.ClientError, err.Error()))
		return
	}
	c.JSON(http.StatusOK, response.Success[any](nil))
}

// ──────────────────────── Chunk ────────────────────────

func (h *Handler) PageChunks(c *gin.Context) {
	docID := c.Param("doc-id")
	page, _ := strconv.Atoi(c.DefaultQuery("current", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	var enabled *int
	if v := c.Query("enabled"); v != "" {
		n, _ := strconv.Atoi(v)
		enabled = &n
	}
	result, err := h.chunk.Page(docID, enabled, page, size)
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Fail[any](errorcode.ClientError, err.Error()))
		return
	}
	c.JSON(http.StatusOK, response.Success(result))
}

func (h *Handler) CreateChunk(c *gin.Context) {
	var req struct {
		Content string `json:"content"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.Fail[any](errorcode.ClientError, "请求参数错误"))
		return
	}
	vo, err := h.chunk.Create(c.Param("doc-id"), req.Content, c.GetString("username"))
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Fail[any](errorcode.ClientError, err.Error()))
		return
	}
	c.JSON(http.StatusOK, response.Success(vo))
}

func (h *Handler) UpdateChunk(c *gin.Context) {
	var req struct {
		Content string `json:"content"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.Fail[any](errorcode.ClientError, "请求参数错误"))
		return
	}
	if err := h.chunk.Update(c.Param("doc-id"), c.Param("chunk-id"), req.Content); err != nil {
		c.JSON(http.StatusBadRequest, response.Fail[any](errorcode.ClientError, err.Error()))
		return
	}
	c.JSON(http.StatusOK, response.Success[any](nil))
}

func (h *Handler) DeleteChunk(c *gin.Context) {
	if err := h.chunk.Delete(c.Param("doc-id"), c.Param("chunk-id")); err != nil {
		c.JSON(http.StatusBadRequest, response.Fail[any](errorcode.ClientError, err.Error()))
		return
	}
	c.JSON(http.StatusOK, response.Success[any](nil))
}

func (h *Handler) EnableChunk(c *gin.Context) {
	// 通过路径末尾判断是 enable 还是 disable
	enabled := !strings.HasSuffix(c.Request.URL.Path, "disable")
	if err := h.chunk.EnableChunk(c.Param("doc-id"), c.Param("chunk-id"), enabled); err != nil {
		c.JSON(http.StatusBadRequest, response.Fail[any](errorcode.ClientError, err.Error()))
		return
	}
	c.JSON(http.StatusOK, response.Success[any](nil))
}

func (h *Handler) BatchEnableChunks(c *gin.Context) {
	enabled := !strings.HasSuffix(c.Request.URL.Path, "disable")
	var req struct {
		IDs []int64 `json:"ids"`
	}
	_ = c.ShouldBindJSON(&req)
	if err := h.chunk.BatchEnable(c.Param("doc-id"), req.IDs, enabled); err != nil {
		c.JSON(http.StatusBadRequest, response.Fail[any](errorcode.ClientError, err.Error()))
		return
	}
	c.JSON(http.StatusOK, response.Success[any](nil))
}
