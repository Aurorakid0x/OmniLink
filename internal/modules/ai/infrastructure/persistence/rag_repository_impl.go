package persistence

import (
	"context"
	"time"

	"OmniLink/internal/modules/ai/domain/rag"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type ragRepositoryImpl struct {
	db *gorm.DB
}

func NewRAGRepository(db *gorm.DB) rag.RAGRepository {
	return &ragRepositoryImpl{db: db}
}

// EnsureKnowledgeBase 使用 upsert 确保知识库存在
func (r *ragRepositoryImpl) EnsureKnowledgeBase(ctx context.Context, kb *rag.AIKnowledgeBase) error {
	// 使用 GORM 的 OnConflict 实现 upsert（对应 MySQL 的 ON DUPLICATE KEY UPDATE）
	// 通过唯一索引 uniq_ai_kb_owner（owner_type, owner_id, kb_type）定位记录
	// 若已存在，则更新 updated_at/status/name
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "owner_type"}, {Name: "owner_id"}, {Name: "kb_type"}},
		DoUpdates: clause.AssignmentColumns([]string{"updated_at", "status", "name"}),
	}).Create(kb).Error
}

// EnsureKnowledgeSource 使用 upsert 确保数据源存在
func (r *ragRepositoryImpl) EnsureKnowledgeSource(ctx context.Context, source *rag.AIKnowledgeSource) error {
	// 通过唯一索引 uniq_ai_source（source_type, source_key）定位记录
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "source_type"}, {Name: "source_key"}},
		DoUpdates: clause.AssignmentColumns([]string{"updated_at", "status", "version", "acl_json"}),
	}).Create(source).Error
}

// CreateChunkAndVectorRecord 使用事务把 chunk 与 vector_record 一起写入，保证原子性
func (r *ragRepositoryImpl) CreateChunkAndVectorRecord(ctx context.Context, chunk *rag.AIKnowledgeChunk, record *rag.AIVectorRecord) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 1) 写入 chunk
		// 说明：chunk_key 有唯一索引；若发生冲突目前直接返回错误（便于上层决定如何处理：跳过/覆盖/重试）
		if err := tx.Create(chunk).Error; err != nil {
			return err
		}

		// 2) 关联 chunk_id（Create 后 chunk.Id 会回填）
		record.ChunkId = chunk.Id

		// 3) 写入向量记录
		if err := tx.Create(record).Error; err != nil {
			return err
		}

		return nil
	})
}

// UpdateVectorStatus 更新向量记录的状态（例如 Milvus 写入成功/失败后回写）
func (r *ragRepositoryImpl) UpdateVectorStatus(ctx context.Context, vectorID string, status int8, errorMsg string) error {
	updates := map[string]interface{}{
		"embed_status": status,
		"updated_at":   time.Now(),
	}
	if errorMsg != "" {
		updates["error_msg"] = errorMsg
	}

	// 成功时记录 embedded_at，便于后续增量/重跑
	if status == 1 {
		updates["embedded_at"] = time.Now()
	}

	return r.db.WithContext(ctx).Model(&rag.AIVectorRecord{}).
		Where("vector_id = ?", vectorID).
		Updates(updates).Error
}
