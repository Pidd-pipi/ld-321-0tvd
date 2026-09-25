import { API_BASE } from '../constants/app.constants';
import { AppException } from '../errors/AppException';
import { logger } from '../logger/logger';
import type { DashboardItem, FarmOverview, TaskActionResult } from '../types/domain';

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

/** 派单/改期/撤单：POST 到对应动作，失败时抛出带后端中文原因的异常。 */
const postTaskAction = async (
  taskId: string,
  action: string,
  fallbackMessage: string,
  payload?: unknown,
): Promise<TaskActionResult> => {
  const response = await fetch(`${API_BASE}/tasks/${taskId}/${action}`, {
    method: 'POST',
    headers: payload ? { 'Content-Type': 'application/json' } : undefined,
    body: payload ? JSON.stringify(payload) : undefined,
  });
  const body = await response.json().catch(() => null);
  if (!response.ok) {
    // 业务失败（如没有空闲农机）：后端消息即失败原因，直接抛给任务卡展示。
    const message = body?.message || fallbackMessage;
    throw new AppException(`TASK_${action.toUpperCase()}_FAILED`, message);
  }
  if (body && typeof body === 'object' && body.code === 0 && body.data) {
    return body.data as TaskActionResult;
  }
  return body as TaskActionResult;
};

/** 一键派单：只占用空闲农机，推荐农机不可用时后端自动改派 */
export const dispatchTask = (taskId: string) =>
  postTaskAction(taskId, 'dispatch', '派单失败');

/** 改期：先选好新的空闲农机再原子切换；targetMachine 为空时由后端自动选择 */
export const rescheduleTask = (taskId: string, targetMachine: string, plannedWindow: string) =>
  postTaskAction(taskId, 'reschedule', '改期失败', { targetMachine, plannedWindow });

/** 撤单：释放农机（无其他在途任务时农机回到空闲） */
export const cancelTask = (taskId: string) =>
  postTaskAction(taskId, 'cancel', '撤单失败');

export const saveItems = (items: DashboardItem[]) => {
  localStorage.setItem('agridispatch.items', JSON.stringify(items));
};
