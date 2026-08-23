// Package testutil содержит общие помощники для тестов.
package testutil

import (
	"testing"

	"github.com/glebarez/sqlite"
	authdomain "github.com/jeogram/messenger/internal/modules/auth/domain"
	calldomain "github.com/jeogram/messenger/internal/modules/calls/domain"
	chatdomain "github.com/jeogram/messenger/internal/modules/chat/domain"
	contactdomain "github.com/jeogram/messenger/internal/modules/contact/domain"
	messagedomain "github.com/jeogram/messenger/internal/modules/message/domain"
	notificationdomain "github.com/jeogram/messenger/internal/modules/notification/domain"
	userdomain "github.com/jeogram/messenger/internal/modules/user/domain"
	"gorm.io/gorm"
)

// NewTestDB открывает in-memory SQLite и выполняет миграции доменных моделей.
func NewTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("не удалось открыть тестовую БД: %v", err)
	}
	if err := db.AutoMigrate(
		&authdomain.User{},
		&userdomain.UserSettings{},
		&userdomain.BlockedUser{},
		&contactdomain.Contact{},
		&chatdomain.Chat{},
		&chatdomain.ChatParticipant{},
		&messagedomain.Message{},
		&messagedomain.ReadReceipt{},
		&messagedomain.Reaction{},
		&messagedomain.PinnedMessage{},
		&notificationdomain.DeviceToken{},
		&notificationdomain.Notification{},
		&calldomain.Call{},
	); err != nil {
		t.Fatalf("миграция не удалась: %v", err)
	}
	return db
}
