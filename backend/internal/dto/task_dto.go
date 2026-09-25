package dto

// DispatchResult 派单结果。
type DispatchResult struct {
	TaskID          string `json:"taskId"`
	Status          string `json:"status"`
	AssignedMachine string `json:"assignedMachine"`
	AssignedDriver  string `json:"assignedDriver"`
	Message         string `json:"message"`
	FailureReason   string `json:"failureReason,omitempty"`
}

// RescheduleRequest 改期请求。
type RescheduleRequest struct {
	PlannedWindow string `json:"plannedWindow" validate:"required"`
	MachineCode   string `json:"machineCode" validate:"omitempty"`
}

// RescheduleResult 改期结果。
type RescheduleResult struct {
	TaskID          string `json:"taskId"`
	Status          string `json:"status"`
	AssignedMachine string `json:"assignedMachine"`
	AssignedDriver  string `json:"assignedDriver"`
	PlannedWindow   string `json:"plannedWindow"`
	Message         string `json:"message"`
	FailureReason   string `json:"failureReason,omitempty"`
}

// CancelResult 撤单结果。
type CancelResult struct {
	TaskID          string `json:"taskId"`
	Status          string `json:"status"`
	AssignedMachine string `json:"assignedMachine"`
	MachineStatus   string `json:"machineStatus,omitempty"`
	Message         string `json:"message"`
}
