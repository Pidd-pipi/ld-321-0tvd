package handler

import (
	"net/http"

	"github.com/agridispatch/agridispatch/internal/constants"
	"github.com/agridispatch/agridispatch/internal/dto"
	"github.com/agridispatch/agridispatch/internal/model"
	"github.com/agridispatch/agridispatch/internal/service"
	"github.com/agridispatch/agridispatch/internal/util"
	"github.com/gin-gonic/gin"
)

// TaskHandler 作业任务调度处理器：派单、改期、撤单。
type TaskHandler struct {
	taskSvc *service.TaskService
}

func NewTaskHandler(taskSvc *service.TaskService) *TaskHandler {
	return &TaskHandler{taskSvc: taskSvc}
}

// Dispatch 一键派单：只占用空闲农机，推荐资源不可用时自动改派空闲农机。
func (h *TaskHandler) Dispatch(c *gin.Context) {
	taskID := c.Param("id")
	if taskID == "" {
		util.Fail(c, http.StatusBadRequest, constants.CodeBadRequest, "task id required")
		return
	}
	task, err := h.taskSvc.Dispatch(c.Request.Context(), taskID)
	if err != nil {
		util.FailError(c, err)
		return
	}
	util.OK(c, toTaskActionResult(task, "派单成功，农机已占用"))
}

// Reschedule 改期：指定/自动选择新的空闲农机后原子切换。
func (h *TaskHandler) Reschedule(c *gin.Context) {
	taskID := c.Param("id")
	if taskID == "" {
		util.Fail(c, http.StatusBadRequest, constants.CodeBadRequest, "task id required")
		return
	}
	var req dto.RescheduleTaskRequest
	// 允许空 body（全部字段可选），仅在 JSON 非法时报 400。
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			util.Fail(c, http.StatusBadRequest, constants.CodeBadRequest, "invalid request body")
			return
		}
	}
	task, err := h.taskSvc.Reschedule(c.Request.Context(), taskID, req.TargetMachine, req.PlannedWindow)
	if err != nil {
		util.FailError(c, err)
		return
	}
	util.OK(c, toTaskActionResult(task, "改期成功，已切换到新农"))
}

// Cancel 撤单：释放农机（无其他在途任务时回到空闲）。
func (h *TaskHandler) Cancel(c *gin.Context) {
	taskID := c.Param("id")
	if taskID == "" {
		util.Fail(c, http.StatusBadRequest, constants.CodeBadRequest, "task id required")
		return
	}
	task, err := h.taskSvc.Cancel(c.Request.Context(), taskID)
	if err != nil {
		util.FailError(c, err)
		return
	}
	util.OK(c, toTaskActionResult(task, "撤单成功，农机已释放"))
}

// toTaskActionResult 将任务模型转为统一响应（直接带最终农机、状态、失败原因）。
func toTaskActionResult(task *model.FarmTask, message string) dto.TaskActionResult {
	return dto.TaskActionResult{
		TaskID:          task.ID,
		Status:          task.Status,
		AssignedMachine: task.AssignedMachine,
		FailReason:      task.FailReason,
		Message:         message,
	}
}
