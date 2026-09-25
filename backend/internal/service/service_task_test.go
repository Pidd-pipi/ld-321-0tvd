package service

import (
	"testing"

	"github.com/agridispatch/agridispatch/internal/model"
)

func TestChooseIdleMachine(t *testing.T) {
	machines := []model.Machine{
		{Code: "NJ-2026-002", Status: "空闲"},
		{Code: "NJ-2026-005", Status: "空闲"},
	}
	tests := []struct {
		name      string
		idle      []model.Machine
		preferred string
		wantCode  string
		wantOK    bool
	}{
		{name: "no idle machine", idle: nil, preferred: "NJ-2026-002", wantCode: "", wantOK: false},
		{name: "preferred available uses it", idle: machines, preferred: "NJ-2026-005", wantCode: "NJ-2026-005", wantOK: true},
		{name: "fallback to first idle when preferred unavailable", idle: machines, preferred: "NJ-2026-001", wantCode: "NJ-2026-002", wantOK: true},
		{name: "empty preferred picks first idle", idle: machines, preferred: "", wantCode: "NJ-2026-002", wantOK: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := chooseIdleMachine(tt.idle, tt.preferred)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if ok && got.Code != tt.wantCode {
				t.Fatalf("got machine %s, want %s", got.Code, tt.wantCode)
			}
		})
	}
}
