package service

import (
	"context"
	"testing"

	"github.com/jeogram/messenger/internal/config"
	chatdomain "github.com/jeogram/messenger/internal/modules/chat/domain"
	chatrepo "github.com/jeogram/messenger/internal/modules/chat/repository"
	"github.com/jeogram/messenger/internal/modules/message/domain"
	msgrepo "github.com/jeogram/messenger/internal/modules/message/repository"
	"github.com/jeogram/messenger/internal/pkg/ws"
	"github.com/jeogram/messenger/internal/testutil"
)

func TestMessageService_SendAndList(t *testing.T) {
	db := testutil.NewTestDB(t)
	chatRepo := chatrepo.NewChatRepository(db)
	msgRepo := msgrepo.NewMessageRepository(db)
	svc := NewMessageService(msgRepo, chatRepo, nil, config.KafkaConfig{}, ws.NewHub(), config.MessageConfig{})

	ctx := context.Background()

	// Создаём приватный чат u1 <-> u2.
	chat := &chatdomain.Chat{Type: chatdomain.ChatTypePrivate}
	if err := chatRepo.Create(ctx, chat, []string{"u1", "u2"}); err != nil {
		t.Fatalf("чат не создан: %v", err)
	}
	chatID := chat.ID

	// u1 отправляет текстовое сообщение.
	msg, err := svc.Send(ctx, "u1", domain.SendMessageRequest{
		ChatID: chatID,
		Type:   string(domain.TypeText),
		Text:   "привет",
	})
	if err != nil {
		t.Fatalf("отправка сообщения не удалась: %v", err)
	}
	if msg.Text != "привет" {
		t.Fatalf("неверный текст: %s", msg.Text)
	}

	// u3 не является участником — доступ запрещён.
	if _, err := svc.Send(ctx, "u3", domain.SendMessageRequest{ChatID: chatID, Type: "text", Text: "x"}); err != ErrForbidden {
		t.Fatalf("ожидался запрет доступа, получено: %v", err)
	}

	// Текстовое сообщение без текста недопустимо.
	if _, err := svc.Send(ctx, "u1", domain.SendMessageRequest{ChatID: chatID, Type: "text"}); err == nil {
		t.Fatal("ожидалась ошибка валидации текста")
	}

	// Список сообщений чата.
	msgs, err := svc.List(ctx, chatID, "u2", 10, 0)
	if err != nil {
		t.Fatalf("список сообщений не удался: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("ожидалось 1 сообщение, получено %d", len(msgs))
	}

	// Редактирование сообщения отправителем.
	edited, err := svc.Edit(ctx, msg.ID, "u1", "привет изменено")
	if err != nil {
		t.Fatalf("редактирование не удалось: %v", err)
	}
	if edited.Text != "привет изменено" {
		t.Fatalf("текст не изменился: %s", edited.Text)
	}

	// Чужое редактирование запрещено.
	if _, err := svc.Edit(ctx, msg.ID, "u2", "хак"); err != ErrForbidden {
		t.Fatalf("ожидался запрет редактирования, получено: %v", err)
	}
}

func TestMessageService_ReactionsPinForwardSearch(t *testing.T) {
	db := testutil.NewTestDB(t)
	chatRepo := chatrepo.NewChatRepository(db)
	msgRepo := msgrepo.NewMessageRepository(db)
	svc := NewMessageService(msgRepo, chatRepo, nil, config.KafkaConfig{}, ws.NewHub(), config.MessageConfig{})
	ctx := context.Background()

	chat := &chatdomain.Chat{Type: chatdomain.ChatTypeGroup}
	if err := chatRepo.Create(ctx, chat, []string{"u1", "u2", "u3"}); err != nil {
		t.Fatalf("чат не создан: %v", err)
	}

	msg, err := svc.Send(ctx, "u1", domain.SendMessageRequest{ChatID: chat.ID, Type: "text", Text: "общий текст сообщения"})
	if err != nil {
		t.Fatalf("отправка: %v", err)
	}

	// Реакции.
	if err := svc.React(ctx, msg.ID, "u2", "👍"); err != nil {
		t.Fatalf("react: %v", err)
	}
	if err := svc.React(ctx, msg.ID, "u3", "👍"); err != nil {
		t.Fatalf("react 2: %v", err)
	}
	reactions, err := svc.ListReactions(ctx, msg.ID)
	if err != nil {
		t.Fatalf("list reactions: %v", err)
	}
	if len(reactions) != 1 || reactions[0].Count != 2 {
		t.Fatalf("ожидалась 1 реакция с count=2, получено: %+v", reactions)
	}
	if err := svc.RemoveReaction(ctx, msg.ID, "u2", "👍"); err != nil {
		t.Fatalf("unreact: %v", err)
	}
	reactions, _ = svc.ListReactions(ctx, msg.ID)
	if reactions[0].Count != 1 {
		t.Fatalf("после удаления ожидался count=1, получено %d", reactions[0].Count)
	}

	// Закрепление.
	if err := svc.Pin(ctx, chat.ID, msg.ID, "u1"); err != nil {
		t.Fatalf("pin: %v", err)
	}
	pinned, err := msgRepo.IsPinned(ctx, msg.ID)
	if err != nil || !pinned {
		t.Fatalf("ожидалось закрепление: %v pinned=%v", err, pinned)
	}

	// Пересылка в другой чат.
	chat2 := &chatdomain.Chat{Type: chatdomain.ChatTypeGroup}
	if err := chatRepo.Create(ctx, chat2, []string{"u1", "u4"}); err != nil {
		t.Fatalf("второй чат: %v", err)
	}
	fwd, err := svc.Forward(ctx, msg.ID, "u1", chat2.ID)
	if err != nil {
		t.Fatalf("forward: %v", err)
	}
	if fwd.Text != "общий текст сообщения" {
		t.Fatalf("неверный текст пересланного: %s", fwd.Text)
	}

	// Поиск по чату и глобальный.
	found, err := svc.SearchInChat(ctx, chat.ID, "u1", "текст", 10)
	if err != nil {
		t.Fatalf("search in chat: %v", err)
	}
	if len(found) != 1 {
		t.Fatalf("ожидался 1 результат в чате, получено %d", len(found))
	}
	global, err := svc.SearchGlobal(ctx, "u1", "текст", 10)
	if err != nil {
		t.Fatalf("search global: %v", err)
	}
	if len(global) != 2 {
		t.Fatalf("ожидалось 2 глобальных результата (оригинал + пересланное), получено %d", len(global))
	}

	// Индикатор печати не должен падать.
	if err := svc.Typing(ctx, chat.ID, "u1"); err != nil {
		t.Fatalf("typing: %v", err)
	}
}

