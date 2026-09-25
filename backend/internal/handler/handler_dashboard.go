package handler

import (
	"net/http"

	"github.com/agridispatch/agridispatch/internal/dto"
	"github.com/agridispatch/agridispatch/internal/service"
	"github.com/agridispatch/agridispatch/internal/util"
	"github.com/gin-gonic/gin"
)

// DashboardHandler 调度看板处理器。
type DashboardHandler struct {
	dashboardSvc *service.DashboardService
}

func NewDashboardHandler(dashboardSvc *service.DashboardService) *DashboardHandler {
	return &DashboardHandler{dashboardSvc: dashboardSvc}
}

// Overview 看板总览。
func (h *DashboardHandler) Overview(c *gin.Context) {
	ov, err := h.dashboardSvc.Overview(c.Request.Context())
	if err != nil {
		util.FailError(c, err)
		return
	}
	util.OK(c, ov)
}

// Dispatch 一键派单。
func (h *DashboardHandler) Dispatch(c *gin.Context) {
	taskID := c.Param("id")
	if taskID == "" {
		util.Fail(c, http.StatusBadRequest, 40000, "task id required")
		return
	}
	res, err := h.dashboardSvc.Dispatch(c.Request.Context(), taskID)
	if err != nil {
		util.FailError(c, err)
		return
	}
	util.OK(c, res)
}

// Reschedule 改期（重新选择农机/作业时间窗）。
func (h *DashboardHandler) Reschedule(c *gin.Context) {
	taskID := c.Param("id")
	if taskID == "" {
		util.Fail(c, http.StatusBadRequest, 40000, "task id required")
		return
	}
	var req dto.RescheduleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		util.Fail(c, http.StatusBadRequest, 40000, "改期参数无效：plannedWindow 必填")
		return
	}
	res, err := h.dashboardSvc.Reschedule(c.Request.Context(), taskID, &req)
	if err != nil {
		util.FailError(c, err)
		return
	}
	util.OK(c, res)
}

// Cancel 撤单。
func (h *DashboardHandler) Cancel(c *gin.Context) {
	taskID := c.Param("id")
	if taskID == "" {
		util.Fail(c, http.StatusBadRequest, 40000, "task id required")
		return
	}
	res, err := h.dashboardSvc.Cancel(c.Request.Context(), taskID)
	if err != nil {
		util.FailError(c, err)
		return
	}
	util.OK(c, res)
}

// ExportReport 作业报表导出。
func (h *DashboardHandler) ExportReport(c *gin.Context) {
	res, err := h.dashboardSvc.ExportReport(c.Request.Context())
	if err != nil {
		util.FailError(c, err)
		return
	}
	util.OK(c, res)
}
