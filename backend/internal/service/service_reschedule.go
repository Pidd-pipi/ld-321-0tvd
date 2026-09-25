package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/agridispatch/agridispatch/internal/constants"
	"github.com/agridispatch/agridispatch/internal/dto"
	apperrors "github.com/agridispatch/agridispatch/internal/errors"
	"github.com/agridispatch/agridispatch/internal/model"
	"github.com/agridispatch/agridispatch/internal/repository"
	"gorm.io/gorm"
)

// Reschedule 改期：先选好新的空闲农机再落库，原任务不会丢失（换机失败则任务保持原状）。
func (s *DashboardService) Reschedule(ctx context.Context, taskID string, req *dto.RescheduleRequest) (*dto.RescheduleResult, error) {
	result := &dto.RescheduleResult{TaskID: taskID}
	err := s.repo.InTx(func(tx *gorm.DB) error {
		task, err := s.repo.LockTask(tx, taskID)
		if err != nil {
			return err
		}
		if task.Status != constants.TaskDispatched && task.Status != constants.TaskRescheduled {
			return apperrors.New(apperrors.CodeTaskState,
				fmt.Sprintf(constants.MsgTaskNotActive, taskID, task.Status))
		}

		// 先锁定新机：
		// - 未指定农机，或指定的就是任务当前农机时，继续沿用当前农机（仅改时间）；
		// - 指定其他农机时，该机必须空闲，否则自动改派另一台空闲农机；
		// - 没有当前农机又未指定时，在空闲机中选一台。
		selector := newMachineSelector(s.repo, s.logger)
		ownMachine := task.AssignedMachine
		var newMachine *model.Machine
		fellBack := false
		if (req.MachineCode == "" || req.MachineCode == ownMachine) && ownMachine != "" {
			newMachine, err = s.repo.LockMachineByCode(tx, ownMachine)
			if err != nil {
				return err
			}
		} else {
			newMachine, fellBack, err = selector.claim(tx, req.MachineCode)
			if err != nil {
				return err
			}
		}
		if req.MachineCode != "" && req.MachineCode != ownMachine && fellBack && newMachine.Code != req.MachineCode {
			// 指定农机不空闲，claim 已自动改派；文案需体现最终农机。
			s.logger.Info("reschedule preferred machine busy, reassigned",
				"taskId", taskID, "preferred", req.MachineCode, "assigned", newMachine.Code)
		}

		oldCode := task.AssignedMachine
		driver := task.AssignedDriver
		if driver == "" {
			driver = task.RecommendedDriver
		}
		if driver == "" {
			driver = constants.DefaultDriverName
		}

		// 新机先占用。
		task.AssignedMachine = newMachine.Code
		task.AssignedDriver = driver
		task.Status = constants.TaskRescheduled
		task.PlannedWindow = req.PlannedWindow
		task.FailureReason = ""
		if err := selector.markWorking(tx, newMachine, task); err != nil {
			return err
		}
		if err := s.repo.SaveTaskTx(tx, task); err != nil {
			return err
		}

		// 再释放原农机：换机时若原机已无其他在途任务则回到空闲；沿用原机则不动。
		machineStatus := newMachine.Status
		if oldCode != "" && oldCode != newMachine.Code {
			machineStatus, err = s.releaseMachineIfIdle(tx, oldCode, taskID)
			if err != nil {
				return err
			}
		}
		s.logger.Info("task rescheduled", "taskId", taskID,
			"oldMachine", oldCode, "newMachine", newMachine.Code, "oldMachineStatus", machineStatus)

		result.Status = task.Status
		result.AssignedMachine = newMachine.Code
		result.AssignedDriver = driver
		result.PlannedWindow = req.PlannedWindow
		result.Message = fmt.Sprintf(constants.MsgRescheduleOK, newMachine.Code)
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
	return result, nil
}

// releaseMachineIfIdle 若指定农机上已无其他在途任务，则释放回空闲；返回该机最终状态。
func (s *DashboardService) releaseMachineIfIdle(tx *gorm.DB, machineCode, excludeTaskID string) (string, error) {
	if machineCode == "" {
		return "", nil
	}
	machine, err := s.repo.LockMachineByCode(tx, machineCode)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return "", nil
		}
		return "", err
	}
	count, err := s.repo.CountActiveTasksOnMachine(tx, machineCode, excludeTaskID)
	if err != nil {
		return "", err
	}
	if count > 0 {
		return machine.Status, nil
	}
	machine.Status = constants.MachineIdle
	machine.CurrentTask = constants.MachineIdleTask
	if err := s.repo.SaveMachineTx(tx, machine); err != nil {
		return "", err
	}
	return machine.Status, nil
}
