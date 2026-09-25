package model

import "time"

// FarmTask 作业任务。
type FarmTask struct {
	ID                 string  `gorm:"primaryKey;size:32" json:"id"`
	Type               string  `gorm:"size:32" json:"type"`
	Field              string  `gorm:"size:64" json:"field"`
	AreaMu             float64 `json:"areaMu"`
	EstimatedHours     float64 `json:"estimatedHours"`
	Status             string  `gorm:"size:20;index" json:"status"`
	Priority           string  `gorm:"size:16" json:"priority"`
	RecommendedMachine string  `gorm:"size:64" json:"recommendedMachine"`
	RecommendedDriver  string  `gorm:"size:64" json:"recommendedDriver"`
	// AssignedMachine 派单/改期最终占用的农机编号；待派单或改期失败时为空。
	AssignedMachine string `gorm:"size:64" json:"assignedMachine"`
	// FailReason 最近一次派单/改期失败原因；成功后置空。
	FailReason    string    `gorm:"size:255" json:"failReason"`
	PlannedWindow string    `gorm:"size:64" json:"plannedWindow"`
	CreatedAt     time.Time `json:"createdAt"`
}
