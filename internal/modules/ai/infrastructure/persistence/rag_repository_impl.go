package persistence

import (
	"context"
	"strings"
	"time"

	"OmniLink/internal/modules/ai/domain/rag"
	"OmniLink/internal/modules/ai/domain/repository"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type ragRepositoryImpl struct {
	db *gorm.DB
}

func NewRAGRepository(db *gorm.DB) repository.RAGRepository {
	return &ragRepositoryImpl{db: db}
}

func (r *ragRepositoryImpl) GetChunkByChunkKey(ctx context.Context, chunkKey string) (*rag.AIKnowledgeChunk, error) {
	var c rag.AIKnowledgeChunk
	err := r.db.WithContext(ctx).Where("chunk_key = ?", chunkKey).Take(&c).Error
	if err == nil {
		return &c, nil
	}
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	return nil, err
}

func (r *ragRepositoryImpl) GetVectorRecordByVectorID(ctx context.Context, vectorID string) (*rag.AIVectorRecord, error) {
	var vr rag.AIVectorRecord
	err := r.db.WithContext(ctx).Where("vector_id = ?", vectorID).Take(&vr).Error
	if err == nil {
		return &vr, nil
	}
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	return nil, err
}

func (r *ragRepositoryImpl) GetVectorRecordByChunkID(ctx context.Context, chunkID int64) (*rag.AIVectorRecord, error) {
	var vr rag.AIVectorRecord
	err := r.db.WithContext(ctx).Where("chunk_id = ?", chunkID).Take(&vr).Error
	if err == nil {
		return &vr, nil
	}
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	return nil, err
}

func (r *ragRepositoryImpl) CreateVectorRecord(ctx context.Context, record *rag.AIVectorRecord) error {
	return r.db.WithContext(ctx).Create(record).Error
}

// EnsureKnowledgeBase 使用 upsert 确保知识库存在，并返回 kb_id
func (r *ragRepositoryImpl) EnsureKnowledgeBase(ctx context.Context, kb *rag.AIKnowledgeBase) (int64, error) {
	// 使用 GORM 的 OnConflict 实现 upsert（对应 MySQL 的 ON DUPLICATE KEY UPDATE）
	// 通过唯一索引 uniq_ai_kb_owner（owner_type, owner_id, kb_type）定位记录
	// 若已存在，则更新 updated_at/status/name
	err := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "owner_type"}, {Name: "owner_id"}, {Name: "kb_type"}},
		DoUpdates: clause.AssignmentColumns([]string{"updated_at", "status", "name"}),
	}).Create(kb).Error
	if err != nil {
		return 0, err
	}
	if kb.Id != 0 {
		return kb.Id, nil
	}

	var existing rag.AIKnowledgeBase
	err = r.db.WithContext(ctx).
		Select("id").
		Where("owner_type = ? AND owner_id = ? AND kb_type = ?", kb.OwnerType, kb.OwnerId, kb.KBType).
		Take(&existing).Error
	if err != nil {
		return 0, err
	}
	kb.Id = existing.Id
	return existing.Id, nil
}

// EnsureKnowledgeSource 使用 upsert 确保数据源存在，并返回 source_id
func (r *ragRepositoryImpl) EnsureKnowledgeSource(ctx context.Context, source *rag.AIKnowledgeSource) (int64, error) {
	if source != nil && strings.TrimSpace(source.ACLJson) == "" {
		source.ACLJson = "{}"
	}
	// 通过唯一索引 uniq_ai_source（kb_id, source_type, source_key, tenant_user_id）定位记录
	err := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "kb_id"}, {Name: "source_type"}, {Name: "source_key"}, {Name: "tenant_user_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"updated_at", "status", "version", "acl_json"}),
	}).Create(source).Error
	if err != nil {
		return 0, err
	}
	if source.Id != 0 {
		return source.Id, nil
	}

	var existing rag.AIKnowledgeSource
	err = r.db.WithContext(ctx).
		Select("id").
		Where("kb_id = ? AND source_type = ? AND source_key = ? AND tenant_user_id = ?", source.KBId, source.SourceType, source.SourceKey, source.TenantUserId).
		Take(&existing).Error
	if err != nil {
		return 0, err
	}
	source.Id = existing.Id
	return existing.Id, nil
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
