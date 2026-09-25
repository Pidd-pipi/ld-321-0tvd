export const APP_NAME = 'AgriDispatch 农机调度';
export const API_BASE = '/api';

// 任务状态枚举
export const TASK_PENDING = '待派单';
export const TASK_DISPATCHED = '已派单';
export const TASK_RESCHEDULED = '已改期';
export const TASK_CANCELLED = '已撤单';
export const TASK_DONE = '已完成';

// 农机状态枚举
export const MACHINE_IDLE = '空闲';
export const MACHINE_WORKING = '作业中';
export const MACHINE_REPAIR = '维修中';

export const STATUS_COLORS: Record<string, string> = {
  [MACHINE_IDLE]: 'success',
  [MACHINE_WORKING]: 'warning',
  [MACHINE_REPAIR]: 'danger',
  [TASK_PENDING]: 'info',
  [TASK_DISPATCHED]: 'warning',
  [TASK_RESCHEDULED]: 'primary',
  [TASK_CANCELLED]: 'info',
  [TASK_DONE]: 'success',
};
