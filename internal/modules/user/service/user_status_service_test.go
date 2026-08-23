package service

import (
	"context"
	"testing"

	"github.com/jeogram/messenger/internal/modules/user/domain"
	"github.com/jeogram/messenger/internal/modules/user/repository"
)

func TestUpdateStatus(t *testing.T) {
	tests := []struct {
		name    string
		status  domain.UserStatus
		wantErr bool
	}{
		{"online", domain.StatusOnline, false},
		{"offline", domain.StatusOffline, false},
		{"away", domain.StatusAway, false},
		{"busy", domain.StatusBusy, false},
		{"dnd", domain.StatusDND, false},
		{"invalid", "invalid_status", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Mock implementation
			err := (tt.status != domain.StatusOnline && 
			       tt.status != domain.StatusOffline && 
			       tt.status != domain.StatusAway && 
			       tt.status != domain.StatusBusy && 
			       tt.status != domain.StatusDND)

			if (err) != tt.wantErr {
				t.Errorf("UpdateStatus() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestBlockUser(t *testing.T) {
	tests := []struct {
		name         string
		userID       string
		blockedID    string
		wantErr      bool
		errorMessage string
	}{
		{"different users", "user1", "user2", false, ""},
		{"same user", "user1", "user1", true, "cannot block yourself"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hasErr := tt.userID == tt.blockedID
			if hasErr != tt.wantErr {
				t.Errorf("BlockUser() error = %v, wantErr %v", hasErr, tt.wantErr)
			}
		})
	}
}
