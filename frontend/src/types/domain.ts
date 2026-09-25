export interface DashboardItem {
  id: string;
  title: string;
  description: string;
  status: string;
  score: number;
}

export interface Machine {
  id: string;
  code: string;
  name: string;
  model: string;
  purchasedAt: string;
  horsepower: number;
  field: string;
  status: string;
  qrCode: string;
  photoUrl: string;
  workHours: number;
  currentTask: string;
}

export interface FarmTask {
  id: string;
  type: string;
  field: string;
  areaMu: number;
  estimatedHours: number;
  status: string;
  priority: string;
  recommendedMachine: string;
  recommendedDriver: string;
  /** 派单/改期最终占用的农机编号；待派单或失败时为空 */
  assignedMachine: string;
  /** 最近一次派单/改期失败原因；成功时为空 */
  failReason: string;
  plannedWindow: string;
}

/** 派单/改期/撤单统一响应 */
export interface TaskActionResult {
  taskId: string;
  status: string;
  assignedMachine: string;
  failReason: string;
  message: string;
}

export interface TrackPoint {
  machineCode: string;
  taskType: string;
  capturedAt: string;
  longitude: number;
  latitude: number;
  speed: number;
  fieldBoundary: string;
}

export interface WorkRecord {
  id: string;
  machineCode: string;
  driverName: string;
  workDate: string;
  taskType: string;
  actualHours: number;
  fuelLiters: number;
  areaMu: number;
  fuelCost: number;
}

export interface MaintenanceReminder {
  id: string;
  machineCode: string;
  title: string;
  dueDate: string;
  remainingHours: number;
  level: string;
  lastServiceRecord: string;
}

export interface Driver {
  id: string;
  name: string;
  licenseNo: string;
  phone: string;
  shift: string;
  restDay: string;
  monthAreaMu: number;
  rating: number;
  status: string;
}

export interface DispatchBoard {
  todayTodos: number;
  idleMachines: number;
  workingMachines: string[];
  dueMaintenance: string[];
  sevenDayAreas: number[];
  trendLabels: string[];
}

export interface FarmOverview {
  items: DashboardItem[];
  machines: Machine[];
  tasks: FarmTask[];
  tracks: TrackPoint[];
  records: WorkRecord[];
  maintenance: MaintenanceReminder[];
  drivers: Driver[];
  board: DispatchBoard;
  stats: {
    totalAreaMu: number;
    totalHours: number;
    fuelCost: number;
  };
}
