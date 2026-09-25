package handler

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/agridispatch/agridispatch/internal/constants"
	"github.com/agridispatch/agridispatch/internal/model"
	"github.com/agridispatch/agridispatch/internal/repository"
	"github.com/agridispatch/agridispatch/internal/service"
	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const apiResponseOK = 0

func setupTaskRouter(t *testing.T) http.Handler {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:e2e?mode=memory&cache=shared"),
		&gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(&model.Machine{}, &model.FarmTask{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	// 两台待派任务都推荐 002，空闲机 002/005 各一台。
	if err := db.Create([]model.Machine{
		{ID: "m1", Code: "NJ-2026-002", Status: constants.MachineIdle, CurrentTask: constants.MachineIdleTaskLabel},
		{ID: "m2", Code: "NJ-2026-005", Status: constants.MachineIdle, CurrentTask: constants.MachineIdleTaskLabel},
	}).Error; err != nil {
		t.Fatalf("seed machines: %v", err)
	}
	if err := db.Create([]model.FarmTask{
		{ID: "t1", Type: "播种", Field: "西坡旱地", Status: constants.TaskPending, RecommendedMachine: "NJ-2026-002"},
		{ID: "t2", Type: "施肥", Field: "南湾稻田", Status: constants.TaskPending, RecommendedMachine: "NJ-2026-002"},
		{ID: "t3", Type: "耕地", Field: "北岭田", Status: constants.TaskPending, RecommendedMachine: "NJ-2026-002"},
	}).Error; err != nil {
		t.Fatalf("seed tasks: %v", err)
	}

	mini, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	t.Cleanup(mini.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mini.Addr()})
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := service.NewTaskService(db, repository.NewTaskRepository(db), repository.NewMachineRepository(db), rdb, log)

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	h := NewTaskHandler(svc)
	engine.POST("/tasks/:id/dispatch", h.Dispatch)
	engine.POST("/tasks/:id/reschedule", h.Reschedule)
	engine.POST("/tasks/:id/cancel", h.Cancel)
	return engine
}

type apiEnvelope struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

func postJSON(t *testing.T, srv *httptest.Server, path, body string) (int, apiEnvelope) {
	t.Helper()
	var reader io.Reader
	if body != "" {
		reader = bytes.NewBufferString(body)
	}
	resp, err := http.Post(srv.URL+path, "application/json", reader)
	if err != nil {
		t.Fatalf("post %s: %v", path, err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var env apiEnvelope
	_ = json.Unmarshal(raw, &env)
	return resp.StatusCode, env
}

func TestTaskDispatchRescheduleCancelFlow(t *testing.T) {
	srv := httptest.NewServer(setupTaskRouter(t))
	defer srv.Close()

	// 两次派单同一推荐机：第一次占 002，第二次必须换台到 005，不能重复占用。
	status, env := postJSON(t, srv, "/tasks/t1/dispatch", "")
	if status != http.StatusOK || env.Code != apiResponseOK {
		t.Fatalf("dispatch t1: status=%d env=%+v", status, env)
	}
	status, env = postJSON(t, srv, "/tasks/t2/dispatch", "")
	if status != http.StatusOK {
		t.Fatalf("dispatch t2: status=%d message=%s", status, env.Message)
	}
	var data struct {
		AssignedMachine string `json:"assignedMachine"`
		Status          string `json:"status"`
		FailReason      string `json:"failReason"`
	}
	if err := json.Unmarshal(env.Data, &data); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if data.AssignedMachine != "NJ-2026-005" {
		t.Fatalf("t2 assigned = %s, want NJ-2026-005（推荐机被占应换台）", data.AssignedMachine)
	}

	// 两台空闲机已被占完，派第三张单：409 且失败原因直接返回。
	status, env = postJSON(t, srv, "/tasks/t3/dispatch", "")
	if status != http.StatusConflict {
		t.Fatalf("dispatch without idle machine status = %d, want 409", status)
	}
	if env.Message != constants.MsgNoIdleMachine {
		t.Fatalf("message = %q, want %q", env.Message, constants.MsgNoIdleMachine)
	}

	// 改期 t2 到 002：002 已被 t1 占用，应 409 且原任务保持 005。
	status, env = postJSON(t, srv, "/tasks/t2/reschedule", `{"targetMachine":"NJ-2026-002"}`)
	if status != http.StatusConflict || env.Message != constants.MsgTargetMachineNotIdle {
		t.Fatalf("reschedule to busy machine: status=%d message=%s", status, env.Message)
	}

	// 撤单 t1 后 002 回到空闲，再改期 t2 到 002 应成功。
	if status, _ = postJSON(t, srv, "/tasks/t1/cancel", ""); status != http.StatusOK {
		t.Fatalf("cancel t1 status = %d", status)
	}
	status, env = postJSON(t, srv, "/tasks/t2/reschedule",
		`{"targetMachine":"NJ-2026-002","plannedWindow":"明日 08:00-16:00"}`)
	if status != http.StatusOK {
		t.Fatalf("reschedule after cancel: status=%d message=%s", status, env.Message)
	}
	_ = json.Unmarshal(env.Data, &data)
	if data.AssignedMachine != "NJ-2026-002" || data.FailReason != "" {
		t.Fatalf("t2 after reschedule = %+v", data)
	}

	// 撤单 t2：002 上已无其他在途任务，回到空闲；任务状态为已撤单。
	status, env = postJSON(t, srv, "/tasks/t2/cancel", "")
	if status != http.StatusOK {
		t.Fatalf("cancel t2 status=%d", status)
	}
	_ = json.Unmarshal(env.Data, &data)
	if data.Status != constants.TaskCanceled {
		t.Fatalf("t2 status = %s, want %s", data.Status, constants.TaskCanceled)
	}
}
