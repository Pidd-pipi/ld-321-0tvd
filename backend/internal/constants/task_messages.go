package constants

// 调度任务相关错误消息（统一维护，禁止业务代码内裸写）。
const (
	MsgTaskDispatchInvalidStatus   = "仅待派单任务可以派单"
	MsgTaskRescheduleInvalidStatus = "仅已派单任务可以改期"
	MsgTaskCancelInvalidStatus     = "仅待派单或已派单任务可以撤单"
	MsgNoIdleMachine               = "当前没有空闲农机，派单失败"
	MsgNoIdleMachineReschedule     = "没有可改期的空闲农机，原任务保持不变"
	MsgTargetMachineNotIdle        = "指定农机不是空闲状态，改期失败"
)
