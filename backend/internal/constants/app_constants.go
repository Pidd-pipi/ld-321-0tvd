package constants

// 应用常量。
const (
	AppName                 = "agridispatch"
	APIVersion              = "v1"
	PageSizeDefault         = 10
	PageSizeMax             = 100
	OverviewCacheKey        = "agridispatch:overview"
	OverviewCacheTTLSeconds = 30
	WSPushIntervalSeconds   = 5
)

// 角色
const (
	RoleAdmin = "admin"
	RoleUser  = "user"
)

// 农机状态
const (
	MachineIdle    = "空闲"
	MachineWorking = "作业中"
	MachineRepair  = "维修中"
)

// MachineIdleTask 空闲农机的当前任务占位文案。
const MachineIdleTask = "可派单"

// DefaultDriverName 任务未推荐驾驶员时的兜底驾驶员。
const DefaultDriverName = "何燕"

// 任务状态
const (
	TaskPending     = "待派单"
	TaskDispatched  = "已派单"
	TaskRescheduled = "已改期"
	TaskCancelled   = "已撤单"
	TaskDone        = "已完成"
)

// ActiveTaskStatuses 仍占用农机的在途任务状态。
var ActiveTaskStatuses = []string{TaskDispatched, TaskRescheduled}

// 派单/改期/撤单提示文案
const (
	MsgDispatchOK       = "系统已按空闲度和驾驶员排班完成派单"
	MsgDispatchFallback = "推荐农机不可用，已改派空闲农机 %s"
	MsgRescheduleOK     = "任务已改期并重新指派农机 %s"
	MsgCancelOK         = "任务已撤单"
	MsgNoIdleMachine    = "当前没有空闲农机可派，请稍后重试"
	MsgPreferredBusy    = "指定农机 %s 当前不可用（非空闲状态）"
	MsgTaskNotPending   = "任务 %s 当前状态为 %s，无法派单"
	MsgTaskNotActive    = "任务 %s 当前状态为 %s，无法改期"
	MsgTaskNotCancel    = "任务 %s 当前状态为 %s，无法撤单"
	MsgMachineMissing   = "农机 %s 不存在"
)

// 错误码
const (
	CodeOK              = 0
	CodeBadRequest      = 40000
	CodeUnauthorized    = 40100
	CodeForbidden       = 40300
	CodeNotFound        = 40400
	CodeConflict        = 40900
	CodeInternalError   = 50000
	CodeTooManyRequests = 42900
)
