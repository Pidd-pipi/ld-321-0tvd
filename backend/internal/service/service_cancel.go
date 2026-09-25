package service

import (
	"context"
	"fmt"

	"github.com/agridispatch/agridispatch/internal/constants"
	"github.com/agridispatch/agridispatch/internal/dto"
	apperrors "github.com/agridispatch/agridispatch/internal/errors"
	"gorm.io/gorm"
)

// Cancel 撤单：任务置为已撤单；若其农机上已无其他在途任务，则农机回到空闲。
func (s *DashboardService) Cancel(ctx context.Context, taskID string) (*dto.CancelResult, error) {
	result := &dto.CancelResult{TaskID: taskID}
	err := s.repo.InTx(func(tx *gorm.DB) error {
		task, err := s.repo.LockTask(tx, taskID)
		if err != nil {
			return err
		}
		if task.Status != constants.TaskPending &&
			task.Status != constants.TaskDispatched &&
			task.Status != constants.TaskRescheduled {
			return apperrors.New(apperrors.CodeTaskState,
				fmt.Sprintf(constants.MsgTaskNotCancel, taskID, task.Status))
		}

		machineCode := task.AssignedMachine
		task.Status = constants.TaskCancelled
		task.FailureReason = ""
		if err := s.repo.SaveTaskTx(tx, task); err != nil {
			return err
		}

		machineStatus := ""
		if machineCode != "" {
			machineStatus, err = s.releaseMachineIfIdle(tx, machineCode, taskID)
			if err != nil {
				return err
			}
		}

		result.Status = task.Status
		result.AssignedMachine = machineCode
		result.MachineStatus = machineStatus
		result.Message = constants.MsgCancelOK
		return nil
	})
	if err != nil {
		return nil, err
	}
	s.Invalidate(ctx)
	s.logger.Info("task cancelled", "taskId", taskID, "machine", result.AssignedMachine,
		"machineStatus", result.MachineStatus)
	return result, nil
}
