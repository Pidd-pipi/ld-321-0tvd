import { API_BASE } from '../constants/app.constants';
import { AppException } from '../errors/AppException';
import { logger } from '../logger/logger';
import type { DashboardItem, FarmOverview, TaskActionResult } from '../types/domain';

// 调用任务操作接口：解包统一响应，失败时把后端错误消息透传给任务卡。
const postTaskAction = async (
  path: string,
  action: string,
  body?: unknown,
): Promise<TaskActionResult> => {
  const response = await fetch(`${API_BASE}${path}`, {
    method: 'POST',
    headers: body ? { 'Content-Type': 'application/json' } : undefined,
    body: body ? JSON.stringify(body) : undefined,
  });
  let payload: { code?: number; message?: string; data?: TaskActionResult } = {};
  try {
    payload = await response.json();
  } catch {
    payload = {};
  }
  if (!response.ok || (payload.code !== undefined && payload.code !== 0)) {
    const reason = payload.message || `${action}失败`;
    logger.error(`${action} failed`, response.status, reason);
    throw new AppException('TASK_ACTION_FAILED', reason);
  }
  // 解包后端统一响应 {code, message, data}
  return payload.data ?? (payload as unknown as TaskActionResult);
};

export const fetchFarmOverview = async (): Promise<FarmOverview> => {
  const response = await fetch(`${API_BASE}/dashboard/overview`);
  if (!response.ok) {
    logger.error('overview request failed', response.status);
    throw new AppException('OVERVIEW_FAILED', '无法加载农机调度看板数据');
  }
  const body = await response.json();
  // 解包后端统一响应 {code, message, data}
  if (body && typeof body === 'object' && body.code === 0 && body.data) {
    return body.data as FarmOverview;
  }
  return body as FarmOverview;
};

// 一键派单：只占用空闲农机，推荐不可用时后端自动改派。
export const dispatchTask = (taskId: string): Promise<TaskActionResult> =>
  postTaskAction(`/tasks/${taskId}/dispatch`, '派单');

// 改期：选定新农机与作业时间窗。
export const rescheduleTask = (
  taskId: string,
  plannedWindow: string,
  machineCode?: string,
): Promise<TaskActionResult> =>
  postTaskAction(`/tasks/${taskId}/reschedule`, '改期', {
    plannedWindow,
    machineCode: machineCode ?? '',
  });

// 撤单：农机无其他在途任务时回到空闲。
export const cancelTask = (taskId: string): Promise<TaskActionResult> =>
  postTaskAction(`/tasks/${taskId}/cancel`, '撤单');

export const saveItems = (items: DashboardItem[]) => {
  localStorage.setItem('agridispatch.items', JSON.stringify(items));
};
