package repository

import (
	"errors"
	"fmt"

	"github.com/agridispatch/agridispatch/internal/constants"
	"github.com/agridispatch/agridispatch/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ErrNotFound 哨兵错误。
var ErrNotFound = errors.New("record not found")

// DashboardRepository 看板数据访问。
type DashboardRepository struct {
	db *gorm.DB
}

func NewDashboardRepository(db *gorm.DB) *DashboardRepository {
	return &DashboardRepository{db: db}
}

// InTx 在事务中执行回调。
func (r *DashboardRepository) InTx(fn func(tx *gorm.DB) error) error {
	if err := r.db.Transaction(fn); err != nil {
		return fmt.Errorf("transaction: %w", err)
	}
	return nil
}

// Overview 组装看板总览。
func (r *DashboardRepository) Overview() (*model.FarmOverview, error) {
	ov := &model.FarmOverview{}
	if err := r.db.Order("score DESC").Find(&ov.Items).Error; err != nil {
		return nil, fmt.Errorf("load items: %w", err)
	}
	if err := r.db.Find(&ov.Machines).Error; err != nil {
		return nil, fmt.Errorf("load machines: %w", err)
	}
	if err := r.db.Find(&ov.Tasks).Error; err != nil {
		return nil, fmt.Errorf("load tasks: %w", err)
	}
	if err := r.db.Order("captured_at DESC").Limit(50).Find(&ov.Tracks).Error; err != nil {
		return nil, fmt.Errorf("load tracks: %w", err)
	}
	if err := r.db.Order("work_date DESC").Find(&ov.Records).Error; err != nil {
		return nil, fmt.Errorf("load records: %w", err)
	}
	if err := r.db.Find(&ov.Maintenance).Error; err != nil {
		return nil, fmt.Errorf("load maintenance: %w", err)
	}
	if err := r.db.Find(&ov.Drivers).Error; err != nil {
		return nil, fmt.Errorf("load drivers: %w", err)
	}
	ov.Board = r.board(ov)
	ov.Stats = r.stats(ov.Records)
	return ov, nil
}

// board 计算调度看板（仅统计在途任务，撤单任务不再计入今日待办）。
func (r *DashboardRepository) board(ov *model.FarmOverview) model.DispatchBoard {
	var idle, working, activeTasks int
	var workingList, dueList []string
	for _, m := range ov.Machines {
		switch m.Status {
		case constants.MachineIdle:
			idle++
		case constants.MachineWorking:
			working++
			workingList = append(workingList, fmt.Sprintf("%s %s", m.Code, m.CurrentTask))
		}
	}
	for _, t := range ov.Tasks {
		if t.Status != constants.TaskCancelled && t.Status != constants.TaskDone {
			activeTasks++
		}
	}
	for _, m := range ov.Maintenance {
		dueList = append(dueList, fmt.Sprintf("%s %s", m.MachineCode, m.Title))
	}
	return model.DispatchBoard{
		TodayTodos:      activeTasks,
		IdleMachines:    idle,
		WorkingMachines: workingList,
		DueMaintenance:  dueList,
		SevenDayAreas:   []int{96, 122, 138, 166, 203, 88, 156},
		TrendLabels:     []string{"5/25", "5/26", "5/27", "5/28", "5/29", "5/30", "5/31"},
	}
}

// stats 汇总作业统计。
func (r *DashboardRepository) stats(records []model.WorkRecord) model.Stats {
	var s model.Stats
	for _, rec := range records {
		s.TotalAreaMu += rec.AreaMu
		s.TotalHours += rec.ActualHours
		s.FuelCost += rec.FuelCost
	}
	return s
}

// FindTask 查找任务。
func (r *DashboardRepository) FindTask(id string) (*model.FarmTask, error) {
	return findTask(r.db, id)
}

func findTask(q *gorm.DB, id string) (*model.FarmTask, error) {
	var t model.FarmTask
	err := q.First(&t, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find task: %w", err)
	}
	return &t, nil
}

// LockTask 在事务中锁定任务行，避免并发派单/改期/撤单互相覆盖。
func (r *DashboardRepository) LockTask(tx *gorm.DB, id string) (*model.FarmTask, error) {
	var t model.FarmTask
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&t, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("lock task: %w", err)
	}
	return &t, nil
}

// SaveTask 更新任务。
func (r *DashboardRepository) SaveTask(t *model.FarmTask) error {
	return saveTask(r.db, t)
}

// SaveTaskTx 在事务中更新任务。
func (r *DashboardRepository) SaveTaskTx(tx *gorm.DB, t *model.FarmTask) error {
	return saveTask(tx, t)
}

func saveTask(q *gorm.DB, t *model.FarmTask) error {
	if err := q.Save(t).Error; err != nil {
		return fmt.Errorf("save task: %w", err)
	}
	return nil
}

// FindMachineByCode 按农机编号查找农机。
func (r *DashboardRepository) FindMachineByCode(code string) (*model.Machine, error) {
	return findMachineByCode(r.db, code)
}

func findMachineByCode(q *gorm.DB, code string) (*model.Machine, error) {
	var m model.Machine
	err := q.First(&m, "code = ?", code).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find machine by code: %w", err)
	}
	return &m, nil
}

// LockMachineByCode 在事务中锁定指定农机行。
func (r *DashboardRepository) LockMachineByCode(tx *gorm.DB, code string) (*model.Machine, error) {
	var m model.Machine
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&m, "code = ?", code).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("lock machine by code: %w", err)
	}
	return &m, nil
}

// LockFirstIdleMachine 在事务中锁定第一台空闲农机（按工时升序，优先使用较空闲农机）。
func (r *DashboardRepository) LockFirstIdleMachine(tx *gorm.DB) (*model.Machine, error) {
	var m model.Machine
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("status = ?", constants.MachineIdle).
		Order("work_hours ASC").
		First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("lock idle machine: %w", err)
	}
	return &m, nil
}

// SaveMachine 更新农机。
func (r *DashboardRepository) SaveMachine(m *model.Machine) error {
	return saveMachine(r.db, m)
}

// SaveMachineTx 在事务中更新农机。
func (r *DashboardRepository) SaveMachineTx(tx *gorm.DB, m *model.Machine) error {
	return saveMachine(tx, m)
}

func saveMachine(q *gorm.DB, m *model.Machine) error {
	if err := q.Save(m).Error; err != nil {
		return fmt.Errorf("save machine: %w", err)
	}
	return nil
}

// CountActiveTasksOnMachine 统计指定农机上除排除任务外的在途任务数量。
func (r *DashboardRepository) CountActiveTasksOnMachine(tx *gorm.DB, machineCode, excludeTaskID string) (int64, error) {
	var count int64
	q := tx.Model(&model.FarmTask{}).
		Where("assigned_machine = ? AND status IN ?", machineCode, constants.ActiveTaskStatuses)
	if excludeTaskID != "" {
		q = q.Where("id <> ?", excludeTaskID)
	}
	if err := q.Count(&count).Error; err != nil {
		return 0, fmt.Errorf("count active tasks on machine: %w", err)
	}
	return count, nil
}

// UserRepository 用户数据访问。
type UserRepository struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) FindByUsername(username string) (*model.User, error) {
	var u model.User
	err := r.db.Where("username = ?", username).First(&u).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find user: %w", err)
	}
	return &u, nil
}
