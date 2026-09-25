package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/agridispatch/agridispatch/internal/constants"
	apperrors "github.com/agridispatch/agridispatch/internal/errors"
	"github.com/agridispatch/agridispatch/internal/model"
	"github.com/agridispatch/agridispatch/internal/repository"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

// TaskService 作业任务调度：派单、改期、撤单。
type TaskService struct {
	db          *gorm.DB
	taskRepo    *repository.TaskRepository
	machineRepo *repository.MachineRepository
	redis       *redis.Client
	logger      *slog.Logger
}

func NewTaskService(
	db *gorm.DB,
	taskRepo *repository.TaskRepository,
	machineRepo *repository.MachineRepository,
	redis *redis.Client,
	logger *slog.Logger,
) *TaskService {
	return &TaskService{
		db:          db,
		taskRepo:    taskRepo,
		machineRepo: machineRepo,
		redis:       redis,
		logger:      logger,
	}
}

// Dispatch 派单：只占用空闲农机；推荐农机不可用时换一台空闲农机。
// 没有空闲农机时任务保持待派单并记录失败原因，返回 ConflictError。
func (s *TaskService) Dispatch(ctx context.Context, taskID string) (*model.FarmTask, error) {
	var result *model.FarmTask
	// bizErr 表示预期内的业务失败（如无空闲机），需要提交事务以落库失败原因。
	var bizErr *apperrors.ConflictError
	txErr := s.db.Transaction(func(tx *gorm.DB) error {
		task, err := s.taskRepo.FindByIDForUpdate(tx, taskID)
		if err != nil {
			return err
		}
		if task.Status != constants.TaskPending {
			return conflict(constants.MsgTaskDispatchInvalidStatus)
		}

		idle, err := s.machineRepo.ListIdleForUpdate(tx, constants.MachineIdle)
		if err != nil {
			return err
		}
		machine, ok := chooseIdleMachine(idle, task.RecommendedMachine)
		if !ok {
			// 无可派资源：保留原任务，记录失败原因，等待下一次派单。
			task.FailReason = constants.MsgNoIdleMachine
			if err := s.taskRepo.Save(tx, task); err != nil {
				return err
			}
			result = task
			bizErr = conflict(constants.MsgNoIdleMachine)
			return nil
		}

		occupy(machine, task)
		if err := s.machineRepo.Save(tx, machine); err != nil {
			return err
		}
		task.AssignedMachine = machine.Code
		task.Status = constants.TaskDispatched
		task.FailReason = ""
		if err := s.taskRepo.Save(tx, task); err != nil {
			return err
		}
		result = task
		s.logger.Info("task dispatched",
			"taskId", taskID, "machine", machine.Code,
			"recommended", task.RecommendedMachine)
		return nil
	})
	s.invalidate(ctx)
	if txErr != nil {
		return nil, txErr
	}
	if bizErr != nil {
		return result, bizErr
	}
	return result, nil
}

// Reschedule 改期：先在事务内锁定并选好新的空闲农机，再释放旧农机。
// 整个过程原子完成，不会把原任务弄丢或改成无农机状态。
func (s *TaskService) Reschedule(ctx context.Context, taskID, targetMachine, plannedWindow string) (*model.FarmTask, error) {
	var result *model.FarmTask
	var bizErr *apperrors.ConflictError
	txErr := s.db.Transaction(func(tx *gorm.DB) error {
		task, err := s.taskRepo.FindByIDForUpdate(tx, taskID)
		if err != nil {
			return err
		}
		if task.Status != constants.TaskDispatched {
			return conflict(constants.MsgTaskRescheduleInvalidStatus)
		}

		var next *model.Machine
		if targetMachine != "" {
			candidate, findErr := s.machineRepo.FindByCodeForUpdate(tx, targetMachine)
			if findErr != nil {
				return findErr
			}
			if candidate.Status != constants.MachineIdle && candidate.Code != task.AssignedMachine {
				task.FailReason = constants.MsgTargetMachineNotIdle
				if err := s.taskRepo.Save(tx, task); err != nil {
					return err
				}
				result = task
				bizErr = conflict(constants.MsgTargetMachineNotIdle)
				return nil
			}
			next = candidate
		} else {
			idle, listErr := s.machineRepo.ListIdleForUpdate(tx, constants.MachineIdle)
			if listErr != nil {
				return listErr
			}
			chosen, ok := chooseIdleMachine(idle, task.RecommendedMachine)
			if !ok {
				task.FailReason = constants.MsgNoIdleMachineReschedule
				if err := s.taskRepo.Save(tx, task); err != nil {
					return err
				}
				result = task
				bizErr = conflict(constants.MsgNoIdleMachineReschedule)
				return nil
			}
			next = chosen
		}

		oldCode := task.AssignedMachine
		if next.Code != oldCode {
			occupy(next, task)
			if err := s.machineRepo.Save(tx, next); err != nil {
				return err
			}
			task.AssignedMachine = next.Code
		}
		if plannedWindow != "" {
			task.PlannedWindow = plannedWindow
		}
		task.FailReason = ""
		if err := s.taskRepo.Save(tx, task); err != nil {
			return err
		}
		// 新农机就绪后再释放旧农机；旧农机没有别的在途任务才回到空闲。
		if oldCode != "" && oldCode != next.Code {
			if err := s.releaseIfNoOtherTask(tx, oldCode, task.ID); err != nil {
				return err
			}
		}
		result = task
		s.logger.Info("task rescheduled",
			"taskId", taskID, "oldMachine", oldCode, "newMachine", next.Code)
		return nil
	})
	s.invalidate(ctx)
	if txErr != nil {
		return nil, txErr
	}
	if bizErr != nil {
		return result, bizErr
	}
	return result, nil
}

// Cancel 撤单：任务置为已撤单；若该农机没有别的在途任务则回到空闲。
func (s *TaskService) Cancel(ctx context.Context, taskID string) (*model.FarmTask, error) {
	var result *model.FarmTask
	err := s.db.Transaction(func(tx *gorm.DB) error {
		task, err := s.taskRepo.FindByIDForUpdate(tx, taskID)
		if err != nil {
			return err
		}
		if task.Status != constants.TaskPending && task.Status != constants.TaskDispatched {
			return conflict(constants.MsgTaskCancelInvalidStatus)
		}

		releasedCode := task.AssignedMachine
		task.Status = constants.TaskCanceled
		task.FailReason = ""
		if err := s.taskRepo.Save(tx, task); err != nil {
			return err
		}
		if releasedCode != "" {
			if err := s.releaseIfNoOtherTask(tx, releasedCode, task.ID); err != nil {
				return err
			}
		}
		result = task
		s.logger.Info("task canceled", "taskId", taskID, "releasedMachine", releasedCode)
		return nil
	})
	s.invalidate(ctx)
	if err != nil {
		return result, err
	}
	return result, nil
}

// releaseIfNoOtherTask 释放农机：除 excludeTaskID 外没有其他在途任务时才回到空闲。
func (s *TaskService) releaseIfNoOtherTask(tx *gorm.DB, machineCode, excludeTaskID string) error {
	machine, err := s.machineRepo.FindByCodeForUpdate(tx, machineCode)
	if err != nil {
		// 农机档案缺失不阻断撤单（任务仍要成功撤掉），仅记录日志。
		if errors.Is(err, repository.ErrNotFound) {
			s.logger.Warn("machine missing when releasing", "machine", machineCode)
			return nil
		}
		return err
	}
	count, err := s.taskRepo.CountInFlightByMachineTx(tx, machineCode, constants.TaskDispatched, excludeTaskID)
	if err != nil {
		return err
	}
	if count > 0 {
		// 还有别的在途任务占用该农机，保持作业中。
		return nil
	}
	machine.Status = constants.MachineIdle
	machine.CurrentTask = constants.MachineIdleTaskLabel
	return s.machineRepo.Save(tx, machine)
}

// invalidate 使看板缓存失效，失败仅告警。
func (s *TaskService) invalidate(ctx context.Context) {
	if err := s.redis.Del(ctx, constants.OverviewCacheKey).Err(); err != nil {
		s.logger.Warn("invalidate overview cache failed", "err", err)
	}
}

// chooseIdleMachine 从空闲农机中选择：优先 preferredCode（推荐农机），
// 不可用时返回编号排序后的第一台空闲农机；无空闲农机返回 ok=false。
// machines 应由调用方在事务内加行锁查出。
func chooseIdleMachine(machines []model.Machine, preferredCode string) (*model.Machine, bool) {
	if len(machines) == 0 {
		return nil, false
	}
	for i := range machines {
		if machines[i].Code == preferredCode {
			return &machines[i], true
		}
	}
	return &machines[0], true
}

// occupy 将农机标记为作业中并挂上任务描述。
func occupy(m *model.Machine, task *model.FarmTask) {
	m.Status = constants.MachineWorking
	m.CurrentTask = fmt.Sprintf("%s %s", task.Type, task.Field)
}

// conflict 构造状态冲突业务错误。
func conflict(message string) *apperrors.ConflictError {
	return &apperrors.ConflictError{Message: message}
}
