package service

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/agridispatch/agridispatch/internal/constants"
	"github.com/agridispatch/agridispatch/internal/dto"
	apperrors "github.com/agridispatch/agridispatch/internal/errors"
	"github.com/agridispatch/agridispatch/internal/model"
	"github.com/agridispatch/agridispatch/internal/repository"
	"github.com/glebarez/sqlite"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

// newTestService 构造内存 SQLite + 空跑 Redis 的服务与种子数据。
func newTestService(t *testing.T) (*DashboardService, *gorm.DB) {
	t.Helper()
	// 每个测试使用独立的内存库，避免共享缓存造成用例间数据污染。
	dsn := "file:" + t.Name() + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql db: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(
		&model.User{}, &model.Machine{}, &model.FarmTask{}, &model.TrackPoint{},
		&model.WorkRecord{}, &model.MaintenanceReminder{}, &model.Driver{}, &model.DashboardItem{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	now := time.Now()
	machines := []model.Machine{
		{ID: "m1", Code: "M-001", Name: "一号机", Status: constants.MachineIdle, WorkHours: 300, CurrentTask: constants.MachineIdleTask, CreatedAt: now},
		{ID: "m2", Code: "M-002", Name: "二号机", Status: constants.MachineIdle, WorkHours: 100, CurrentTask: constants.MachineIdleTask, CreatedAt: now},
		{ID: "m3", Code: "M-003", Name: "三号机", Status: constants.MachineWorking, WorkHours: 200, CurrentTask: "其他在途任务", CreatedAt: now},
	}
	if err := db.Create(&machines).Error; err != nil {
		t.Fatalf("seed machines: %v", err)
	}
	tasks := []model.FarmTask{
		{ID: "t1", Type: "耕地", Field: "北岭", Status: constants.TaskPending, RecommendedMachine: "M-001", RecommendedDriver: "周明", CreatedAt: now},
		{ID: "t2", Type: "播种", Field: "南湾", Status: constants.TaskPending, RecommendedMachine: "M-001", RecommendedDriver: "何燕", CreatedAt: now},
		{ID: "t3", Type: "施肥", Field: "西坡", Status: constants.TaskPending, RecommendedMachine: "M-003", RecommendedDriver: "刘强", CreatedAt: now},
		{ID: "t4", Type: "收割", Field: "东河", Status: constants.TaskDispatched, RecommendedMachine: "M-003", AssignedMachine: "M-003", AssignedDriver: "刘强", CreatedAt: now},
	}
	if err := db.Create(&tasks).Error; err != nil {
		t.Fatalf("seed tasks: %v", err)
	}

	repo := repository.NewDashboardRepository(db)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewDashboardService(repo, redis.NewClient(&redis.Options{Addr: "127.0.0.1:0"}), logger), db
}

func TestDispatchOnlyClaimsIdleMachine(t *testing.T) {
	svc, db := newTestService(t)
	ctx := context.Background()

	// t1 派给推荐机 M-001。
	res, err := svc.Dispatch(ctx, "t1")
	if err != nil {
		t.Fatalf("dispatch t1: %v", err)
	}
	if res.AssignedMachine != "M-001" || res.Status != constants.TaskDispatched {
		t.Fatalf("unexpected dispatch result: %+v", res)
	}

	// t2 同样推荐 M-001，但该机已被占用，必须自动改派另一台空闲机，而不是重复占用。
	res2, err := svc.Dispatch(ctx, "t2")
	if err != nil {
		t.Fatalf("dispatch t2: %v", err)
	}
	if res2.AssignedMachine == "M-001" {
		t.Fatalf("machine M-001 was double-booked: %+v", res2)
	}
	if res2.AssignedMachine != "M-002" {
		t.Fatalf("expected fallback to M-002, got %q", res2.AssignedMachine)
	}

	// 无空闲机后再派 t3（推荐机 M-003 作业中），应失败且失败原因落库。
	_, err = svc.Dispatch(ctx, "t3")
	var bizErr *apperrors.BusinessError
	if !errors.As(err, &bizErr) || bizErr.Code != apperrors.CodeNoIdleMachine {
		t.Fatalf("expected no idle machine error, got %v", err)
	}
	var t3 model.FarmTask
	if err := db.First(&t3, "id = ?", "t3").Error; err != nil {
		t.Fatalf("load t3: %v", err)
	}
	if t3.FailureReason == "" || t3.Status != constants.TaskPending {
		t.Fatalf("failure reason not persisted / task changed: %+v", t3)
	}
}

func TestRescheduleKeepsTaskAndFreesOnlyIdleMachine(t *testing.T) {
	svc, db := newTestService(t)
	ctx := context.Background()

	// t1、t2 依次占用 M-001、M-002。
	if _, err := svc.Dispatch(ctx, "t1"); err != nil {
		t.Fatalf("dispatch t1: %v", err)
	}
	if _, err := svc.Dispatch(ctx, "t2"); err != nil {
		t.Fatalf("dispatch t2: %v", err)
	}

	// 把 t1 从 M-001 改到 M-002 会失败（M-002 已占用且无其他空闲机），任务不得丢失。
	_, err := svc.Reschedule(ctx, "t1", &dto.RescheduleRequest{PlannedWindow: "明日", MachineCode: "M-002"})
	var bizErr *apperrors.BusinessError
	if !errors.As(err, &bizErr) {
		t.Fatalf("expected conflict error, got %v", err)
	}
	var t1 model.FarmTask
	if err := db.First(&t1, "id = ?", "t1").Error; err != nil {
		t.Fatalf("load t1: %v", err)
	}
	if t1.AssignedMachine != "M-001" || t1.Status != constants.TaskDispatched {
		t.Fatalf("original task was lost after failed reschedule: %+v", t1)
	}

	// 撤掉 t2 释放 M-002。
	if _, err := svc.Cancel(ctx, "t2"); err != nil {
		t.Fatalf("cancel t2: %v", err)
	}
	var m2 model.Machine
	if err := db.First(&m2, "code = ?", "M-002").Error; err != nil {
		t.Fatalf("load m2: %v", err)
	}
	if m2.Status != constants.MachineIdle {
		t.Fatalf("M-002 should be idle after its only task cancelled, got %q", m2.Status)
	}

	// 现在 t1 改期到 M-002：原机 M-001 回到空闲，任务保留并显示最终农机。
	res, err := svc.Reschedule(ctx, "t1", &dto.RescheduleRequest{PlannedWindow: "后日 08:00", MachineCode: "M-002"})
	if err != nil {
		t.Fatalf("reschedule t1: %v", err)
	}
	if res.AssignedMachine != "M-002" || res.Status != constants.TaskRescheduled || res.PlannedWindow != "后日 08:00" {
		t.Fatalf("unexpected reschedule result: %+v", res)
	}
	if err := db.First(&t1, "id = ?", "t1").Error; err != nil {
		t.Fatalf("reload t1: %v", err)
	}
	if t1.AssignedMachine != "M-002" {
		t.Fatalf("final machine not persisted: %+v", t1)
	}
	var m1 model.Machine
	if err := db.First(&m1, "code = ?", "M-001").Error; err != nil {
		t.Fatalf("load m1: %v", err)
	}
	if m1.Status != constants.MachineIdle || m1.CurrentTask != constants.MachineIdleTask {
		t.Fatalf("M-001 should return to idle after reschedule, got %q/%q", m1.Status, m1.CurrentTask)
	}
}

func TestRescheduleOntoSameMachineKeepsItWorking(t *testing.T) {
	svc, db := newTestService(t)
	ctx := context.Background()

	if _, err := svc.Dispatch(ctx, "t1"); err != nil {
		t.Fatalf("dispatch t1: %v", err)
	}

	// 不换农机，仅改时间窗：农机必须保持作业中，任务不得丢失。
	res, err := svc.Reschedule(ctx, "t1", &dto.RescheduleRequest{PlannedWindow: "下周一 08:00", MachineCode: ""})
	if err != nil {
		t.Fatalf("reschedule t1: %v", err)
	}
	if res.AssignedMachine != "M-001" {
		t.Fatalf("machine changed unexpectedly: %+v", res)
	}
	var m1 model.Machine
	if err := db.First(&m1, "code = ?", "M-001").Error; err != nil {
		t.Fatalf("load m1: %v", err)
	}
	if m1.Status != constants.MachineWorking {
		t.Fatalf("M-001 should stay working on same-machine reschedule, got %q", m1.Status)
	}
}

func TestCancelKeepsMachineBusyWhenOtherActiveTask(t *testing.T) {
	svc, db := newTestService(t)
	ctx := context.Background()

	// t4 占用 M-003；再人为给 M-003 增加一个在途任务 t5。
	t5 := model.FarmTask{ID: "t5", Type: "耕地", Field: "北岭", Status: constants.TaskDispatched, AssignedMachine: "M-003", AssignedDriver: "周明"}
	if err := db.Create(&t5).Error; err != nil {
		t.Fatalf("create t5: %v", err)
	}

	// 撤掉 t4，但 M-003 上还有 t5，农机必须保持作业中。
	if _, err := svc.Cancel(ctx, "t4"); err != nil {
		t.Fatalf("cancel t4: %v", err)
	}
	var m3 model.Machine
	if err := db.First(&m3, "code = ?", "M-003").Error; err != nil {
		t.Fatalf("load m3: %v", err)
	}
	if m3.Status != constants.MachineWorking {
		t.Fatalf("M-003 should stay working with t5 active, got %q", m3.Status)
	}

	// 再撤 t5，M-003 才回到空闲。
	if _, err := svc.Cancel(ctx, "t5"); err != nil {
		t.Fatalf("cancel t5: %v", err)
	}
	if err := db.First(&m3, "code = ?", "M-003").Error; err != nil {
		t.Fatalf("reload m3: %v", err)
	}
	if m3.Status != constants.MachineIdle {
		t.Fatalf("M-003 should be idle after last task cancelled, got %q", m3.Status)
	}
}
