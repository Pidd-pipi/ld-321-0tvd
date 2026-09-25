package dto

// TaskActionResult 派单/改期/撤单统一返回。
type TaskActionResult struct {
	TaskID          string `json:"taskId"`
	Status          string `json:"status"`
	AssignedMachine string `json:"assignedMachine"`
	FailReason      string `json:"failReason"`
	Message         string `json:"message"`
}

// RescheduleTaskRequest 改期请求。
// TargetMachine 为空时由系统从空闲农机中自动选择；PlannedWindow 为空表示不改作业时间。
type RescheduleTaskRequest struct {
	TargetMachine string `json:"targetMachine"`
	PlannedWindow string `json:"plannedWindow"`
}
