package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/agridispatch/agridispatch/internal/constants"
	"github.com/agridispatch/agridispatch/internal/dto"
	apperrors "github.com/agridispatch/agridispatch/internal/errors"
	"github.com/agridispatch/agridispatch/internal/model"
	"github.com/agridispatch/agridispatch/internal/repository"
	"gorm.io/gorm"
)

// machineSelector 农机选择逻辑：推荐/指定农机优先，不可用时换一台空闲农机。
type machineSelector struct {
	repo   *repository.DashboardRepository
	logger *slog.Logger
}

func newMachineSelector(repo *repository.DashboardRepository, logger *slog.Logger) *machineSelector {
	return &machineSelector{repo: repo, logger: logger}
}

// claim 在事务中占用一台农机。preferredCode 非空且空闲时优先使用；
// 推荐农机不存在或不空闲时回退到另一台空闲农机；均无空闲机时返回 apperrors.BusinessError。
// 返回最终占用的农机以及是否发生了回退。
func (ms *machineSelector) claim(tx *gorm.DB, preferredCode string) (machine *model.Machine, fellBack bool, err error) {
	if preferredCode != "" {
		preferred, findErr := ms.repo.LockMachineByCode(tx, preferredCode)
		if findErr != nil && !errors.Is(findErr, repository.ErrNotFound) {
			return nil, false, findErr
		}
		if preferred != nil {
			if preferred.Status == constants.MachineIdle {
				return preferred, false, nil
			}
			fellBack = true
			ms.logger.Info("preferred machine busy, fallback to another idle machine",
				"preferred", preferredCode, "status", preferred.Status)
		} else {
			fellBack = true
			ms.logger.Info("preferred machine missing, fallback to another idle machine",
				"preferred", preferredCode)
		}
	}
	idle, err := ms.repo.LockFirstIdleMachine(tx)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, false, apperrors.NewNoIdleMachine()
		}
		return nil, false, err
	}
	return idle, fellBack, nil
}

// markWorking 将农机标记为作业中并绑定任务。
func (ms *machineSelector) markWorking(tx *gorm.DB, m *model.Machine, task *model.FarmTask) error {
	m.Status = constants.MachineWorking
	m.CurrentTask = taskCurrentTask(task)
	return ms.repo.SaveMachineTx(tx, m)
}

// Dispatch 派单：只占用空闲农机，推荐资源不可用时换一台空闲农机。
func (s *DashboardService) Dispatch(ctx context.Context, taskID string) (*dto.DispatchResult, error) {
	result := &dto.DispatchResult{TaskID: taskID}
	err := s.repo.InTx(func(tx *gorm.DB) error {
		task, err := s.repo.LockTask(tx, taskID)
		if err != nil {
			return err
		}
		if task.Status != constants.TaskPending {
			return apperrors.New(apperrors.CodeTaskState,
				fmt.Sprintf(constants.MsgTaskNotPending, taskID, task.Status))
		}

		selector := newMachineSelector(s.repo, s.logger)
		machine, fellBack, err := selector.claim(tx, task.RecommendedMachine)
		if err != nil {
			return err
		}

		driver := task.RecommendedDriver
		if driver == "" {
			driver = constants.DefaultDriverName
		}
		task.AssignedMachine = machine.Code
		task.AssignedDriver = driver
		task.Status = constants.TaskDispatched
		task.FailureReason = ""
		if err := s.repo.SaveTaskTx(tx, task); err != nil {
			return err
		}
		if err := selector.markWorking(tx, machine, task); err != nil {
			return err
		}

		result.Status = task.Status
		result.AssignedMachine = machine.Code
		result.AssignedDriver = driver
		if fellBack {
			result.Message = fmt.Sprintf(constants.MsgDispatchFallback, machine.Code)
		} else {
			result.Message = constants.MsgDispatchOK
		}
		return nil
	})
	if err != nil {
		var bizErr *apperrors.BusinessError
		if errors.As(err, &bizErr) && bizErr.Code == apperrors.CodeNoIdleMachine {
			s.persistFailure(ctx, taskID, bizErr.Message)
			result.FailureReason = bizErr.Message
		}
		return nil, err
	}
	s.Invalidate(ctx)
	s.logger.Info("task dispatched", "taskId", taskID, "machine", result.AssignedMachine, "driver", result.AssignedDriver)
	return result, nil
}
