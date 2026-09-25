package model

import "time"

// Machine 农机档案。
type Machine struct {
	ID          string    `gorm:"primaryKey;size:32" json:"id"`
	Code        string    `gorm:"size:32;uniqueIndex" json:"code"`
	Name        string    `gorm:"size:64" json:"name"`
	Model       string    `gorm:"size:64" json:"model"`
	PurchasedAt string    `gorm:"size:32" json:"purchasedAt"`
	Horsepower  int       `json:"horsepower"`
	Field       string    `gorm:"size:64" json:"field"`
	Status      string    `gorm:"size:20;index" json:"status"`
	QRCode      string    `gorm:"size:64" json:"qrCode"`
	PhotoURL    string    `gorm:"size:255" json:"photoUrl"`
	WorkHours   float64   `json:"workHours"`
	CurrentTask string    `gorm:"size:64" json:"currentTask"`
	CreatedAt   time.Time `json:"createdAt"`
}

// TrackPoint 农机实时轨迹点。
type TrackPoint struct {
	ID            uint      `gorm:"primaryKey" json:"-"`
	MachineCode   string    `gorm:"size:32;index" json:"machineCode"`
	TaskType      string    `gorm:"size:32" json:"taskType"`
	CapturedAt    string    `gorm:"size:32" json:"capturedAt"`
	Longitude     float64   `json:"longitude"`
	Latitude      float64   `json:"latitude"`
	Speed         float64   `json:"speed"`
	FieldBoundary string    `gorm:"size:64" json:"fieldBoundary"`
	CreatedAt     time.Time `json:"-"`
}
