package service

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/agridispatch/agridispatch/internal/constants"
	apperrors "github.com/agridispatch/agridispatch/internal/errors"
	"github.com/agridispatch/agridispatch/internal/model"
	"github.com/agridispatch/agridispatch/internal/repository"
	"github.com/alicebob/miniredis/v2"
	"github.com/glebarez/sqlite"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// fixture 为单个测试准备独立的内存 SQLite + miniredis + TaskService。
type fixture struct {
	svc *TaskService
	db  *gorm.DB
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	// 每个测试使用独立的内存库名；单连接保证内存库生命周期内数据可见。
	dsn := "file:" + t.Name() + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql db: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(&model.Machine{}, &model.FarmTask{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	mini, err := miniredis.Run()
	if err != nil {
		t.Fatalf("start miniredis: %v", err)
	}
	t.Cleanup(func() {
		mini.Close()
		_ = sqlDB.Close()
	})
	rdb := redis.NewClient(&redis.Options{Addr: mini.Addr()})
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := NewTaskService(db, repository.NewTaskRepository(db), repository.NewMachineRepository(db), rdb, log)
	return &fixture{svc: svc, db: db}
}

// seed 写入典型调度场景：
// 001 作业中（t4 在途）、002 空闲、003 作业中（t3/t5 两个在途任务）、004 维修中、005 空闲；
// t1 待派单推荐 002；t2 待派单推荐 003（资源不可用，需换台）；
// t4 已派单独占 001；t6 已完成；t7 待派单（用于无空闲场景）。
func (f *fixture) seed(t *testing.T) {
	t.Helper()
	machines := []model.Machine{
		{ID: "m1", Code: "NJ-2026-001", Status: constants.MachineWorking, CurrentTask: "旋耕 东河田"},
		{ID: "m2", Code: "NJ-2026-002", Status: constants.MachineIdle, CurrentTask: constants.MachineIdleTaskLabel},
		{ID: "m3", Code: "NJ-2026-003", Status: constants.MachineWorking, CurrentTask: "施肥 南湾稻田"},
		{ID: "m4", Code: "NJ-2026-004", Status: constants.MachineRepair, CurrentTask: "液压检修"},
		{ID: "m5", Code: "NJ-2026-005", Status: constants.MachineIdle, CurrentTask: constants.MachineIdleTaskLabel},
	}
	if err := f.db.Create(&machines).Error; err != nil {
		t.Fatalf("seed machines: %v", err)
	}
	tasks := []model.FarmTask{
		{ID: "t1", Type: "播种", Field: "西坡旱地", Status: constants.TaskPending, RecommendedMachine: "NJ-2026-002"},
		{ID: "t2", Type: "耕地", Field: "北岭田", Status: constants.TaskPending, RecommendedMachine: "NJ-2026-003"},
		{ID: "t3", Type: "施肥", Field: "南湾稻田", Status: constants.TaskDispatched, AssignedMachine: "NJ-2026-003", PlannedWindow: "今日 14:00-20:00"},
		{ID: "t4", Type: "旋耕", Field: "东河田", Status: constants.TaskDispatched, AssignedMachine: "NJ-2026-001", PlannedWindow: "明日 06:00-12:00"},
		{ID: "t5", Type: "打药", Field: "南湾稻田", Status: constants.TaskDispatched, AssignedMachine: "NJ-2026-003"},
		{ID: "t6", Type: "收割", Field: "东河麦田", Status: constants.TaskDone, AssignedMachine: "NJ-2026-001"},
		{ID: "t7", Type: "转运", Field: "仓库", Status: constants.TaskPending, RecommendedMachine: "NJ-2026-002"},
	}
	if err := f.db.Create(&tasks).Error; err != nil {
		t.Fatalf("seed tasks: %v", err)
	}
}

func (f *fixture) mustTask(t *testing.T, id string) model.FarmTask {
	t.Helper()
	var task model.FarmTask
	if err := f.db.First(&task, "id = ?", id).Error; err != nil {
		t.Fatalf("load task %s: %v", id, err)
	}
	return task
}

func (f *fixture) mustMachine(t *testing.T, code string) model.Machine {
	t.Helper()
	var m model.Machine
	if err := f.db.First(&m, "code = ?", code).Error; err != nil {
		t.Fatalf("load machine %s: %v", code, err)
	}
	return m
}

func TestDispatchOccupiesRecommendedIdleMachine(t *testing.T) {
	f := newFixture(t)
	f.seed(t)

	task, err := f.svc.Dispatch(context.Background(), "t1")
	if err != nil {
		t.Fatalf("dispatch t1: %v", err)
	}
	if task == nil || task.AssignedMachine != "NJ-2026-002" || task.Status != constants.TaskDispatched || task.FailReason != "" {
		t.Fatalf("task after dispatch = %+v", task)
	}
	if m := f.mustMachine(t, "NJ-2026-002"); m.Status != constants.MachineWorking {
		t.Fatalf("machine 002 status = %s, want %s", m.Status, constants.MachineWorking)
	}
}

func TestDispatchFallsBackToAnotherIdleMachine(t *testing.T) {
	f := newFixture(t)
	f.seed(t)

	// t2 推荐的 003 正在作业，空闲机只有 002/005，应自动换台而不是强占 003。
	task, err := f.svc.Dispatch(context.Background(), "t2")
	if err != nil {
		t.Fatalf("dispatch t2: %v", err)
	}
	if task.AssignedMachine == "NJ-2026-003" {
		t.Fatalf("occupied non-idle recommended machine 003: %+v", task)
	}
	if task.AssignedMachine != "NJ-2026-002" {
		t.Fatalf("got assigned %s, want fallback NJ-2026-002", task.AssignedMachine)
	}

	// 第二张同推荐机的任务派单时，002 已被占用，应换剩下的 005；不会重复占用。
	other, err := f.svc.Dispatch(context.Background(), "t1")
	if err != nil {
		t.Fatalf("dispatch t1: %v", err)
	}
	if other.AssignedMachine != "NJ-2026-005" {
		t.Fatalf("got assigned %s, want NJ-2026-005", other.AssignedMachine)
	}

	// 003 上的在途任务不受影响。
	if m := f.mustMachine(t, "NJ-2026-003"); m.Status != constants.MachineWorking {
		t.Fatalf("machine 003 status = %s", m.Status)
	}
}

func TestDispatchWithoutIdleMachineRecordsFailReason(t *testing.T) {
	f := newFixture(t)
	f.seed(t)
	// 让全部农机不可派：002/005 置为维修（001/003 已在作业）。
	if err := f.db.Model(&model.Machine{}).Where("code IN ?", []string{"NJ-2026-002", "NJ-2026-005"}).
		Update("status", constants.MachineRepair).Error; err != nil {
		t.Fatalf("block machines: %v", err)
	}

	_, err := f.svc.Dispatch(context.Background(), "t1")
	var conflict *apperrors.ConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("err = %v, want ConflictError", err)
	}
	task := f.mustTask(t, "t1")
	if task.Status != constants.TaskPending {
		t.Fatalf("task status = %s, want %s（任务不能丢）", task.Status, constants.TaskPending)
	}
	if task.AssignedMachine != "" || task.FailReason != constants.MsgNoIdleMachine {
		t.Fatalf("task = %+v", task)
	}
}

func TestDispatchRejectsNonPendingTask(t *testing.T) {
	f := newFixture(t)
	f.seed(t)
	for _, id := range []string{"t3", "t6"} {
		if _, err := f.svc.Dispatch(context.Background(), id); !errors.As(err, new(*apperrors.ConflictError)) {
			t.Fatalf("dispatch %s err = %v, want ConflictError", id, err)
		}
	}
	if _, err := f.svc.Dispatch(context.Background(), "missing"); !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("dispatch missing err = %v, want ErrNotFound", err)
	}
}

func TestRescheduleSwitchesMachineAtomically(t *testing.T) {
	f := newFixture(t)
	f.seed(t)

	// t3 在 003 上（另有 t5 也在 003），改期到空闲的 002 并改时间。
	task, err := f.svc.Reschedule(context.Background(), "t3", "NJ-2026-002", "明日 08:00-16:00")
	if err != nil {
		t.Fatalf("reschedule t3: %v", err)
	}
	if task.AssignedMachine != "NJ-2026-002" || task.Status != constants.TaskDispatched {
		t.Fatalf("task = %+v", task)
	}
	if task.PlannedWindow != "明日 08:00-16:00" {
		t.Fatalf("planned window = %s", task.PlannedWindow)
	}
	if m := f.mustMachine(t, "NJ-2026-002"); m.Status != constants.MachineWorking {
		t.Fatalf("002 status = %s, want 作业中", m.Status)
	}
	// 003 还有 t5 在途，必须保持作业中。
	if m := f.mustMachine(t, "NJ-2026-003"); m.Status != constants.MachineWorking {
		t.Fatalf("003 status = %s, want 作业中（t5 仍在途）", m.Status)
	}
}

func TestRescheduleAutoPickReleasesOldMachine(t *testing.T) {
	f := newFixture(t)
	f.seed(t)

	// t4 独占 001；不指定目标机时自动选择空闲 002，旧机 001 无其他在途任务应回到空闲。
	task, err := f.svc.Reschedule(context.Background(), "t4", "", "后日 06:00-12:00")
	if err != nil {
		t.Fatalf("reschedule t4: %v", err)
	}
	if task.AssignedMachine != "NJ-2026-002" {
		t.Fatalf("assigned = %s, want NJ-2026-002", task.AssignedMachine)
	}
	if m := f.mustMachine(t, "NJ-2026-001"); m.Status != constants.MachineIdle {
		t.Fatalf("old machine 001 status = %s, want 空闲", m.Status)
	}
	if m := f.mustMachine(t, "NJ-2026-002"); m.Status != constants.MachineWorking {
		t.Fatalf("new machine 002 status = %s, want 作业中", m.Status)
	}
}

func TestRescheduleFailureKeepsOriginalTask(t *testing.T) {
	f := newFixture(t)
	f.seed(t)

	// 指定正在作业的 003 给 t4：改期必须失败，原任务原样保留。
	_, err := f.svc.Reschedule(context.Background(), "t4", "NJ-2026-003", "")
	var conflict *apperrors.ConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("err = %v, want ConflictError", err)
	}
	task := f.mustTask(t, "t4")
	if task.AssignedMachine != "NJ-2026-001" || task.Status != constants.TaskDispatched {
		t.Fatalf("original task lost/changed: %+v", task)
	}
	if task.FailReason != constants.MsgTargetMachineNotIdle {
		t.Fatalf("failReason = %q", task.FailReason)
	}
	if m := f.mustMachine(t, "NJ-2026-001"); m.Status != constants.MachineWorking {
		t.Fatalf("001 should stay 作业中, got %s", m.Status)
	}

	// 非已派单任务不能改期。
	if _, err := f.svc.Reschedule(context.Background(), "t1", "", ""); !errors.As(err, new(*apperrors.ConflictError)) {
		t.Fatalf("reschedule pending err = %v, want ConflictError", err)
	}
}

func TestRescheduleWithoutIdleMachineKeepsOriginal(t *testing.T) {
	f := newFixture(t)
	f.seed(t)
	if err := f.db.Model(&model.Machine{}).Where("code IN ?", []string{"NJ-2026-002", "NJ-2026-005"}).
		Update("status", constants.MachineRepair).Error; err != nil {
		t.Fatalf("block machines: %v", err)
	}
	// t3 独占 003（先把同机的 t5 撤掉），再自动改期时已无空闲机。
	if _, err := f.svc.Cancel(context.Background(), "t5"); err != nil {
		t.Fatalf("cancel t5: %v", err)
	}
	// 撤单会让 003 回到空闲，一并置为维修，确保没有任何可派农机。
	if err := f.db.Model(&model.Machine{}).Where("code = ?", "NJ-2026-003").
		Update("status", constants.MachineRepair).Error; err != nil {
		t.Fatalf("block machine 003: %v", err)
	}
	if _, err := f.svc.Reschedule(context.Background(), "t3", "", ""); !errors.As(err, new(*apperrors.ConflictError)) {
		t.Fatalf("expected conflict")
	}
	task := f.mustTask(t, "t3")
	if task.AssignedMachine != "NJ-2026-003" {
		t.Fatalf("task changed to %s, original must be kept", task.AssignedMachine)
	}
}

func TestCancelReleasesOnlyWhenNoOtherInFlightTask(t *testing.T) {
	f := newFixture(t)
	f.seed(t)

	// 003 上有 t3、t5 两个在途任务（模拟历史上同一台被派两单）。
	if _, err := f.svc.Cancel(context.Background(), "t3"); err != nil {
		t.Fatalf("cancel t3: %v", err)
	}
	if m := f.mustMachine(t, "NJ-2026-003"); m.Status != constants.MachineWorking {
		t.Fatalf("003 status = %s, want 作业中（t5 仍在途）", m.Status)
	}
	if task := f.mustTask(t, "t3"); task.Status != constants.TaskCanceled {
		t.Fatalf("t3 status = %s, want 已撤单", task.Status)
	}

	if _, err := f.svc.Cancel(context.Background(), "t5"); err != nil {
		t.Fatalf("cancel t5: %v", err)
	}
	if m := f.mustMachine(t, "NJ-2026-003"); m.Status != constants.MachineIdle {
		t.Fatalf("003 status = %s, want 空闲（最后在途任务已撤）", m.Status)
	}

	// 独占农机的任务撤单后立即回到空闲。
	if _, err := f.svc.Cancel(context.Background(), "t4"); err != nil {
		t.Fatalf("cancel t4: %v", err)
	}
	if m := f.mustMachine(t, "NJ-2026-001"); m.Status != constants.MachineIdle {
		t.Fatalf("001 status = %s, want 空闲", m.Status)
	}
}

func TestCancelRejectsInvalidStatus(t *testing.T) {
	f := newFixture(t)
	f.seed(t)
	if _, err := f.svc.Cancel(context.Background(), "t6"); !errors.As(err, new(*apperrors.ConflictError)) {
		t.Fatalf("cancel done task err = %v, want ConflictError", err)
	}
}
