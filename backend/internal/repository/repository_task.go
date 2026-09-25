package repository

import (
	"errors"
	"fmt"

	"github.com/agridispatch/agridispatch/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// TaskRepository 作业任务数据访问。
type TaskRepository struct {
	db *gorm.DB
}

func NewTaskRepository(db *gorm.DB) *TaskRepository {
	return &TaskRepository{db: db}
}

// FindByID 查找任务。
func (r *TaskRepository) FindByID(id string) (*model.FarmTask, error) {
	return r.FindByIDTx(r.db, id)
}

// FindByIDTx 事务版本：查找任务。
func (r *TaskRepository) FindByIDTx(tx *gorm.DB, id string) (*model.FarmTask, error) {
	var t model.FarmTask
	err := tx.First(&t, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find task: %w", err)
	}
	return &t, nil
}

// FindByIDForUpdate 事务内加行锁查找任务。
func (r *TaskRepository) FindByIDForUpdate(tx *gorm.DB, id string) (*model.FarmTask, error) {
	var t model.FarmTask
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&t, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find task for update: %w", err)
	}
	return &t, nil
}

// Save 更新任务。
func (r *TaskRepository) Save(tx *gorm.DB, t *model.FarmTask) error {
	if err := tx.Save(t).Error; err != nil {
		return fmt.Errorf("save task: %w", err)
	}
	return nil
}

// CountInFlightByMachineTx 事务内统计某台农机仍在途（inFlightStatus，通常为“已派单”）的任务数。
// excludeTaskID 用于排除当前正在撤单/改期的任务本身。
func (r *TaskRepository) CountInFlightByMachineTx(tx *gorm.DB, machineCode, inFlightStatus, excludeTaskID string) (int64, error) {
	var count int64
	q := tx.Model(&model.FarmTask{}).
		Where("assigned_machine = ? AND status = ?", machineCode, inFlightStatus)
	if excludeTaskID != "" {
		q = q.Where("id <> ?", excludeTaskID)
	}
	if err := q.Count(&count).Error; err != nil {
		return 0, fmt.Errorf("count in-flight tasks by machine: %w", err)
	}
	return count, nil
}
