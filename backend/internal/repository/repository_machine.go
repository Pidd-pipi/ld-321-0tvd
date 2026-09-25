package repository

import (
	"errors"
	"fmt"

	"github.com/agridispatch/agridispatch/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// MachineRepository 农机数据访问。
type MachineRepository struct {
	db *gorm.DB
}

func NewMachineRepository(db *gorm.DB) *MachineRepository {
	return &MachineRepository{db: db}
}

// ListByStatus 按状态查询全部农机。
func (r *MachineRepository) ListByStatus(status string) ([]model.Machine, error) {
	return r.ListByStatusTx(r.db, status)
}

// ListByStatusTx 事务版本：按状态查询全部农机。
func (r *MachineRepository) ListByStatusTx(tx *gorm.DB, status string) ([]model.Machine, error) {
	var machines []model.Machine
	if err := tx.Where("status = ?", status).Order("code ASC").Find(&machines).Error; err != nil {
		return nil, fmt.Errorf("list machines by status: %w", err)
	}
	return machines, nil
}

// ListIdleForUpdate 事务内加行锁加载指定状态的全部农机（派单/改期选择空闲资源使用）。
func (r *MachineRepository) ListIdleForUpdate(tx *gorm.DB, status string) ([]model.Machine, error) {
	var machines []model.Machine
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("status = ?", status).
		Order("code ASC").
		Find(&machines).Error; err != nil {
		return nil, fmt.Errorf("list idle machines for update: %w", err)
	}
	return machines, nil
}

// FindByCode 按农机编号查找农机。
func (r *MachineRepository) FindByCode(code string) (*model.Machine, error) {
	return r.FindByCodeTx(r.db, code)
}

// FindByCodeTx 事务版本：按农机编号查找农机。
func (r *MachineRepository) FindByCodeTx(tx *gorm.DB, code string) (*model.Machine, error) {
	var m model.Machine
	err := tx.First(&m, "code = ?", code).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find machine by code: %w", err)
	}
	return &m, nil
}

// FindByCodeForUpdate 事务内加行锁按编号查找农机，找不到返回 ErrNotFound。
func (r *MachineRepository) FindByCodeForUpdate(tx *gorm.DB, code string) (*model.Machine, error) {
	var m model.Machine
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&m, "code = ?", code).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find machine by code for update: %w", err)
	}
	return &m, nil
}

// Save 更新农机。
func (r *MachineRepository) Save(tx *gorm.DB, m *model.Machine) error {
	if err := tx.Save(m).Error; err != nil {
		return fmt.Errorf("save machine: %w", err)
	}
	return nil
}
