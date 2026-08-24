package service

import (
	"context"
	"testing"

	"github.com/jeogram/messenger/internal/modules/chat/domain"
	"github.com/jeogram/messenger/internal/modules/chat/repository"
	"github.com/jeogram/messenger/internal/testutil"
)

func TestChatService_PrivateAndGroup(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := repository.NewChatRepository(db)
	svc := NewChatService(repo, nil)
	ctx := context.Background()

	// Приватный чат создаётся и повторный вызов возвращает тот же чат.
	c1, err := svc.CreatePrivateChat(ctx, "u1", "u2")
	if err != nil {
		t.Fatalf("приватный чат не создан: %v", err)
	}
	c2, err := svc.CreatePrivateChat(ctx, "u1", "u2")
	if err != nil {
		t.Fatalf("повторный приватный чат не удался: %v", err)
	}
	if c1.ID != c2.ID {
		t.Fatal("повторный вызов должен вернуть тот же чат")
	}
	if len(c1.Participants) != 2 {
		t.Fatalf("ожидалось 2 участника, получено %d", len(c1.Participants))
	}

	// Чат с самим собой запрещён.
	if _, err := svc.CreatePrivateChat(ctx, "u1", "u1"); err == nil {
		t.Fatal("ожидалась ошибка создания чата с самим собой")
	}

	// Групповой чат.
	g, err := svc.CreateGroupChat(ctx, "u1", domain.CreateGroupChatRequest{
		Title:          "My Group",
		ParticipantIDs: []string{"u2", "u3"},
	})
	if err != nil {
		t.Fatalf("групповой чат не создан: %v", err)
	}
	if g.Type != domain.ChatTypeGroup {
		t.Fatalf("неверный тип чата: %s", g.Type)
	}
	if len(g.Participants) != 3 {
		t.Fatalf("ожидалось 3 участника, получено %d", len(g.Participants))
	}

	// Список чатов пользователя u1.
	chats, err := svc.ListChats(ctx, "u1")
	if err != nil {
		t.Fatalf("список чатов не удался: %v", err)
	}
	if len(chats) != 2 {
		t.Fatalf("ожидалось 2 чата, получено %d", len(chats))
	}
}

func TestChatService_AdminPermissions(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := repository.NewChatRepository(db)
	svc := NewChatService(repo, nil)
	ctx := context.Background()

	g, err := svc.CreateGroupChat(ctx, "owner", domain.CreateGroupChatRequest{
		Title:          "Админский чат",
		ParticipantIDs: []string{"member1", "member2"},
	})
	if err != nil {
		t.Fatalf("группа не создана: %v", err)
	}
	chatID := g.ID

	// Только владелец может назначать админом.
	if err := svc.Promote(ctx, chatID, "owner", "member1"); err != nil {
		t.Fatalf("назначение админом не удалось: %v", err)
	}
	// Участник без прав не может назначать админом.
	if err := svc.Promote(ctx, chatID, "member2", "member1"); err == nil {
		t.Fatal("участник не должен назначать админом")
	}
	// Админ (member1) может менять название чата.
	if _, err := svc.UpdateChat(ctx, chatID, "member1", "Новое название", ""); err != nil {
		t.Fatalf("админ не может обновить чат: %v", err)
	}
	// Обычный участник (member2) не может менять название чата.
	if _, err := svc.UpdateChat(ctx, chatID, "member2", "Хак", ""); err == nil {
		t.Fatal("участник не должен менять чат")
	}
	// Владелец понижает админа.
	if err := svc.Demote(ctx, chatID, "owner", "member1"); err != nil {
		t.Fatalf("понижение не удалось: %v", err)
	}
	// Теперь member1 снова обычный участник — не может менять чат.
	if _, err := svc.UpdateChat(ctx, chatID, "member1", "Снова хак", ""); err == nil {
		t.Fatal("пониженный админ не должен менять чат")
	}
	// Удаление участника владельцем.
	if err := svc.RemoveParticipant(ctx, chatID, "owner", "member2"); err != nil {
		t.Fatalf("удаление участника не удалось: %v", err)
	}
	// Удалить владельца нельзя.
	if err := svc.RemoveParticipant(ctx, chatID, "owner", "owner"); err == nil {
		t.Fatal("владельца нельзя удалить")
	}
}
