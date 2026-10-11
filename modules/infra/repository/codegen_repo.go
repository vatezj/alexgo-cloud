package repository

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"

	"alexGo-cloud/modules/infra/model"
)

// CodegenRepository 配置表读写。删除一律硬删（spec §9.3）；
// 批量字段保存只写配置字段，快照由同步链路维护（§9.4）。
type CodegenRepository interface {
	ListTables(ctx context.Context, page, size int) ([]*model.CodegenTable, int64, error)
	ListAllTables(ctx context.Context) ([]*model.CodegenTable, error)
	GetTable(ctx context.Context, id uint64) (*model.CodegenTable, error)
	CreateTable(ctx context.Context, t *model.CodegenTable, cols []*model.CodegenColumn) error
	UpdateTable(ctx context.Context, t *model.CodegenTable) error
	DeleteTable(ctx context.Context, id uint64) error
	ListColumns(ctx context.Context, tableID uint64) ([]*model.CodegenColumn, error)
	SaveColumns(ctx context.Context, tableID uint64, cols []*model.CodegenColumn) error
	ImportedTableNames(ctx context.Context) ([]string, error)
	// ApplySync 单事务落同步差异：插新行、标 deprecated、刷快照、刷表注释。
	ApplySync(ctx context.Context, tableID uint64, tableComment string, diff model.SyncDiff) error
}

type codegenRepo struct{ db *gorm.DB }

func NewCodegenRepository(db *gorm.DB) CodegenRepository { return &codegenRepo{db: db} }

func (r *codegenRepo) ListTables(ctx context.Context, page, size int) ([]*model.CodegenTable, int64, error) {
	var total int64
	if err := r.db.WithContext(ctx).Model(&model.CodegenTable{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []*model.CodegenTable
	err := r.db.WithContext(ctx).
		Order("id desc").
		Offset((page - 1) * size).Limit(size).
		Find(&items).Error
	return items, total, err
}

func (r *codegenRepo) ListAllTables(ctx context.Context) ([]*model.CodegenTable, error) {
	var items []*model.CodegenTable
	err := r.db.WithContext(ctx).Order("id asc").Find(&items).Error
	return items, err
}

func (r *codegenRepo) GetTable(ctx context.Context, id uint64) (*model.CodegenTable, error) {
	var item model.CodegenTable
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

// CreateTable 事务内建表+字段；table_name 唯一键冲突原样上抛（导入去重的最后防线）。
func (r *codegenRepo) CreateTable(ctx context.Context, t *model.CodegenTable, cols []*model.CodegenColumn) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(t).Error; err != nil {
			return err
		}
		for _, c := range cols {
			c.TableID = t.ID
			if err := tx.Create(c).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *codegenRepo) UpdateTable(ctx context.Context, t *model.CodegenTable) error {
	return r.db.WithContext(ctx).Save(t).Error
}

// DeleteTable 硬删 + 级联删字段（§9.3）。
func (r *codegenRepo) DeleteTable(ctx context.Context, id uint64) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("table_id = ?", id).Delete(&model.CodegenColumn{}).Error; err != nil {
			return err
		}
		return tx.Where("id = ?", id).Delete(&model.CodegenTable{}).Error
	})
}

func (r *codegenRepo) ListColumns(ctx context.Context, tableID uint64) ([]*model.CodegenColumn, error) {
	var items []*model.CodegenColumn
	err := r.db.WithContext(ctx).
		Where("table_id = ?", tableID).
		Order("sort_order asc, id asc").
		Find(&items).Error
	return items, err
}

// configFields 是用户可编辑、同步不得覆盖的字段（§9.4 的「配置字段」）。
var configFields = []string{
	"html_type", "list_enable", "form_enable", "query_enable",
	"query_operation", "list_required", "form_required", "dict_type", "sort_order",
}

// SaveColumns 按 id 批量覆盖配置字段；table_id 不匹配的行报错（防跨表写）。
func (r *codegenRepo) SaveColumns(ctx context.Context, tableID uint64, cols []*model.CodegenColumn) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, c := range cols {
			res := tx.Model(&model.CodegenColumn{}).
				Where("id = ? AND table_id = ?", c.ID, tableID).
				Select(configFields).
				Updates(c)
			if res.Error != nil {
				return res.Error
			}
			if res.RowsAffected == 0 {
				return fmt.Errorf("column %d not in table %d", c.ID, tableID)
			}
		}
		return nil
	})
}

func (r *codegenRepo) ImportedTableNames(ctx context.Context) ([]string, error) {
	var names []string
	err := r.db.WithContext(ctx).
		Model(&model.CodegenTable{}).
		Pluck("table_name", &names).Error
	return names, err
}

// snapshotFields 是同步允许刷新的快照字段（§9.4）。
var snapshotFields = []string{
	"type", "comment", "go_type", "json_name",
	"is_pk", "auto_increment", "nullable", "deprecated", "updated_at",
}

// ApplySync 单事务落库（§9.4）：插新行 → 标 deprecated → 刷快照 → 刷表注释。
func (r *codegenRepo) ApplySync(ctx context.Context, tableID uint64, tableComment string, diff model.SyncDiff) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, c := range diff.Added {
			c.ID = 0
			c.TableID = tableID
			if err := tx.Create(c).Error; err != nil {
				return err
			}
		}
		if len(diff.Deprecated) > 0 {
			if err := tx.Model(&model.CodegenColumn{}).
				Where("id IN ? AND table_id = ?", diff.Deprecated, tableID).
				Update("deprecated", true).Error; err != nil {
				return err
			}
		}
		for _, c := range diff.Refresh {
			if err := tx.Model(&model.CodegenColumn{}).
				Where("id = ? AND table_id = ?", c.ID, tableID).
				Select(snapshotFields).
				Updates(c).Error; err != nil {
				return err
			}
		}
		return tx.Model(&model.CodegenTable{}).
			Where("id = ?", tableID).
			Updates(map[string]any{"table_comment": tableComment, "updated_at": time.Now()}).Error
	})
}
