package knowledge

import (
	"crypto/sha256"
	"errors"
	"fmt"
)

// ChunkService 管理 Chunk 的 CRUD 和启用/禁用。
type ChunkService struct {
	chunkRepo ChunkRepo
	docRepo   DocRepo
}

func NewChunkService(chunkRepo ChunkRepo, docRepo DocRepo) *ChunkService {
	return &ChunkService{chunkRepo: chunkRepo, docRepo: docRepo}
}

// Page 分页查询 Chunk，enabled 为 nil 时不过滤。
func (s *ChunkService) Page(docIDStr string, enabled *int, page, size int) (*PageResult[KnowledgeChunkVO], error) {
	docID, err := parseID(docIDStr)
	if err != nil {
		return nil, errors.New("文档ID非法")
	}
	if _, err := s.docRepo.FindByID(docID); err != nil {
		return nil, errors.New("文档不存在")
	}

	chunks, total, err := s.chunkRepo.PageByDocID(docID, enabled, page, size)
	if err != nil {
		return nil, err
	}
	records := make([]KnowledgeChunkVO, 0, len(chunks))
	for _, c := range chunks {
		records = append(records, *toChunkVO(c))
	}

	return &PageResult[KnowledgeChunkVO]{Total: total, Records: records}, nil
}

// Create 手动新增 Chunk，自动分配 chunkIndex。
func (s *ChunkService) Create(docIDStr, content, createdBy string) (*KnowledgeChunkVO, error) {
	docID, err := parseID(docIDStr)
	if err != nil {
		return nil, errors.New("文档ID非法")
	}
	doc, err := s.docRepo.FindByID(docID)
	if err != nil {
		return nil, errors.New("文档不存在")
	}
	if content == "" {
		return nil, errors.New("Chunk 内容不能为空")
	}

	maxIdx, err := s.chunkRepo.MaxIndexByDocID(docID)
	if err != nil {
		return nil, err
	}
	chunk := &KnowledgeChunk{
		KbID:        doc.KbID,
		DocID:       docID,
		ChunkIndex:  maxIdx + 1,
		Content:     content,
		ContentHash: hashContent(content),
		CharCount:   len([]rune(content)), // rune 正确统计 Unicode 字符数
		Enabled:     1,
		CreatedBy:   createdBy,
	}
	if err := s.chunkRepo.Create(chunk); err != nil {
		return nil, err
	}
	_ = s.docRepo.UpdateChunkCount(docID, 1)

	return toChunkVO(*chunk), nil
}

// Update 修改 Chunk 内容，同步更新 hash 和字符数。
func (s *ChunkService) Update(docIDStr, chunkIDStr, content string) error {
	if _, err := parseID(docIDStr); err != nil {
		return errors.New("文档ID非法")
	}
	chunkID, err := parseID(chunkIDStr)
	if err != nil {
		return errors.New("ChunkID非法")
	}
	chunk, err := s.chunkRepo.FindByID(chunkID)
	if err != nil {
		return errors.New("Chunk不存在")
	}
	chunk.Content = content
	chunk.ContentHash = hashContent(content)
	chunk.CharCount = len([]rune(content))

	return s.chunkRepo.Update(chunk)
}

// Delete 删除单个 Chunk 并更新文档计数。
func (s *ChunkService) Delete(docIDStr, chunkIDStr string) error {
	docID, err := parseID(docIDStr)
	if err != nil {
		return errors.New("文档ID非法")
	}
	chunkID, err := parseID(chunkIDStr)
	if err != nil {
		return errors.New("ChunkID非法")
	}
	if err := s.chunkRepo.Delete(chunkID); err != nil {
		return err
	}
	_ = s.docRepo.UpdateChunkCount(docID, -1)

	return nil
}

// EnableChunk 启用或禁用单个 Chunk。
func (s *ChunkService) EnableChunk(docIDStr, chunkIDStr string, enabled bool) error {
	if _, err := parseID(docIDStr); err != nil {
		return errors.New("文档ID非法")
	}
	chunkID, err := parseID(chunkIDStr)
	if err != nil {
		return errors.New("ChunkID非法")
	}
	return s.chunkRepo.SetEnabled(chunkID, boolToInt(enabled))
}

// BatchEnable 批量启用/禁用，ids 为空时对整个文档生效。
func (s *ChunkService) BatchEnable(docIDStr string, ids []int64, enabled bool) error {
	docID, err := parseID(docIDStr)
	if err != nil {
		return errors.New("文档ID非法")
	}
	return s.chunkRepo.SetEnabledByDocID(docID, ids, boolToInt(enabled))
}

// ──────────────────────────────────────────────────────────

func hashContent(content string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(content)))
}

func toChunkVO(c KnowledgeChunk) *KnowledgeChunkVO {
	return &KnowledgeChunkVO{
		ID:         fmt.Sprintf("%d", c.ID),
		KbID:       fmt.Sprintf("%d", c.KbID),
		DocID:      fmt.Sprintf("%d", c.DocID),
		ChunkIndex: c.ChunkIndex,
		Content:    c.Content,
		CharCount:  c.CharCount,
		TokenCount: c.TokenCount,
		Enabled:    c.Enabled == 1,
		CreatedAt:  c.CreatedAt,
	}
}
