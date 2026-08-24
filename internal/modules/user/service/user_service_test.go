package service

import (
	"context"
	"testing"

	"github.com/jeogram/messenger/internal/modules/auth/repository"
	chatrepo "github.com/jeogram/messenger/internal/modules/chat/repository"
	contactrepo "github.com/jeogram/messenger/internal/modules/contact/repository"
	notificationrepo "github.com/jeogram/messenger/internal/modules/notification/repository"
	userrepo "github.com/jeogram/messenger/internal/modules/user/repository"
	"github.com/jeogram/messenger/internal/testutil"
)

func TestUserService_BlockUnblock(t *testing.T) {
	db := testutil.NewTestDB(t)
	userRepo := repository.NewUserRepository(db)
	settingsRepo := userrepo.NewSettingsRepository(db)
	contactRepo := contactrepo.NewContactRepository(db)
	chatRepo := chatrepo.NewChatRepository(db)
	deviceRepo := notificationrepo.NewDeviceRepository(db)
	svc := NewUserService(userRepo, settingsRepo, contactRepo, chatRepo, deviceRepo)
	ctx := context.Background()

	// u1 не может заблокировать самого себя.
	if err := svc.Block(ctx, "u1", "u1"); err == nil {
		t.Fatal("ожидалась ошибка блокировки себя")
	}

	if err := svc.Block(ctx, "u1", "u2"); err != nil {
		t.Fatalf("block: %v", err)
	}
	blocks, err := svc.ListBlocks(ctx, "u1")
	if err != nil {
		t.Fatalf("list blocks: %v", err)
	}
	if len(blocks) != 1 || blocks[0].BlockedID != "u2" {
		t.Fatalf("ожидалась 1 блокировка u2, получено: %+v", blocks)
	}
	if err := svc.Unblock(ctx, "u1", "u2"); err != nil {
		t.Fatalf("unblock: %v", err)
	}
	blocks, _ = svc.ListBlocks(ctx, "u1")
	if len(blocks) != 0 {
		t.Fatalf("ожидалось 0 блокировок после unblock, получено %d", len(blocks))
	}
}
