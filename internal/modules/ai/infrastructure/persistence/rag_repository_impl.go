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

	if status == 1 {
		updates["embedded_at"] = time.Now()
	}

	return r.db.WithContext(ctx).Model(&rag.AIVectorRecord{}).
		Where("vector_id = ?", vectorID).
		Updates(updates).Error
}

func (r *ragRepositoryImpl) GetKnowledgeSource(ctx context.Context, kbID int64, tenantUserID, sourceType, sourceKey string) (*rag.AIKnowledgeSource, error) {
	var src rag.AIKnowledgeSource
	err := r.db.WithContext(ctx).
		Where("kb_id = ? AND tenant_user_id = ? AND source_type = ? AND source_key = ?", kbID, strings.TrimSpace(tenantUserID), strings.TrimSpace(sourceType), strings.TrimSpace(sourceKey)).
		Take(&src).Error
	if err == nil {
		return &src, nil
	}
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	return nil, err
}

func (r *ragRepositoryImpl) ListVectorIDsBySourceID(ctx context.Context, sourceID int64) ([]string, error) {
	if sourceID <= 0 {
		return []string{}, nil
	}
	var ids []string
	err := r.db.WithContext(ctx).
		Table("ai_vector_record AS vr").
		Joins("JOIN ai_knowledge_chunk AS c ON c.id = vr.chunk_id").
		Where("c.source_id = ?", sourceID).
		Pluck("vr.vector_id", &ids).Error
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return []string{}, nil
	}
	out := make([]string, 0, len(ids))
	seen := map[string]struct{}{}
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out, nil
}

func (r *ragRepositoryImpl) DeleteChunksAndVectorRecordsBySourceID(ctx context.Context, sourceID int64) error {
	if sourceID <= 0 {
		return nil
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		sub := tx.Model(&rag.AIKnowledgeChunk{}).Select("id").Where("source_id = ?", sourceID)
		if err := tx.Where("chunk_id IN (?)", sub).Delete(&rag.AIVectorRecord{}).Error; err != nil {
			return err
		}
		if err := tx.Where("source_id = ?", sourceID).Delete(&rag.AIKnowledgeChunk{}).Error; err != nil {
			return err
		}
		return nil
	})
}

func (r *ragRepositoryImpl) UpdateKnowledgeSourceStatus(ctx context.Context, sourceID int64, status int8) error {
	if sourceID <= 0 {
		return nil
	}
	return r.db.WithContext(ctx).Model(&rag.AIKnowledgeSource{}).
		Where("id = ?", sourceID).
		Updates(map[string]any{"status": status, "updated_at": time.Now()}).Error
}

func (r *ragRepositoryImpl) GetChunksByIDs(ctx context.Context, chunkIDs []int64) (map[int64]*rag.AIKnowledgeChunk, error) {
	if len(chunkIDs) == 0 {
		return map[int64]*rag.AIKnowledgeChunk{}, nil
	}

	var chunks []rag.AIKnowledgeChunk
	err := r.db.WithContext(ctx).
		Where("id IN ? AND status = ?", chunkIDs, rag.CommonStatusEnabled).
		Find(&chunks).Error
	if err != nil {
		return nil, err
	}

	result := make(map[int64]*rag.AIKnowledgeChunk, len(chunks))
	for i := range chunks {
		result[chunks[i].Id] = &chunks[i]
	}
	return result, nil
}

func (r *ragRepositoryImpl) GetSourcesByIDs(ctx context.Context, sourceIDs []int64) (map[int64]*rag.AIKnowledgeSource, error) {
	if len(sourceIDs) == 0 {
		return map[int64]*rag.AIKnowledgeSource{}, nil
	}

	var sources []rag.AIKnowledgeSource
	err := r.db.WithContext(ctx).
		Where("id IN ? AND status = ?", sourceIDs, rag.CommonStatusEnabled).
		Find(&sources).Error
	if err != nil {
		return nil, err
	}

	result := make(map[int64]*rag.AIKnowledgeSource, len(sources))
	for i := range sources {
		result[sources[i].Id] = &sources[i]
	}
	return result, nil
}
