package service

import (
	"context"
	"errors"
	"time"

	"github.com/jeogram/messenger/internal/modules/chat/domain"
	"github.com/jeogram/messenger/internal/modules/chat/repository"
)

type ChatInfoService struct {
	chatRepo *repository.ChatRepository
}

func NewChatInfoService(chatRepo *repository.ChatRepository) *ChatInfoService {
	return &ChatInfoService{chatRepo: chatRepo}
}

func (s *ChatInfoService) GetChatInfo(ctx context.Context, chatID, userID string) (*domain.ChatInfo, error) {
	chat, err := s.chatRepo.GetByID(ctx, chatID)
	if err != nil {
		return nil, err
	}

	memberCount, _ := s.chatRepo.GetMemberCount(ctx, chatID)
	messageCount, _ := s.chatRepo.GetMessageCount(ctx, chatID)

	return &domain.ChatInfo{
		ID:           chat.ID,
		Name:         chat.Name,
		Description:  "",
		AvatarURL:    chat.AvatarURL,
		Type:         chat.Type,
		CreatedAt:    chat.CreatedAt,
		UpdatedAt:    chat.UpdatedAt,
		MemberCount:  memberCount,
		MessageCount: messageCount,
	}, nil
}

func (s *ChatInfoService) UpdateChatInfo(ctx context.Context, chatID, userID string, req domain.UpdateChatInfoRequest) (*domain.ChatInfo, error) {
	chat, err := s.chatRepo.GetByID(ctx, chatID)
	if err != nil {
		return nil, err
	}

	// Check if user is owner or admin
	isAdmin, _ := s.chatRepo.IsAdmin(ctx, chatID, userID)
	if chat.CreatedBy != userID && !isAdmin {
		return nil, errors.New("only chat owner or admin can update info")
	}

	chat.Name = req.Name
	chat.AvatarURL = req.AvatarURL
	chat.UpdatedAt = time.Now()

	if err := s.chatRepo.Update(ctx, chat); err != nil {
		return nil, err
	}

	memberCount, _ := s.chatRepo.GetMemberCount(ctx, chatID)
	messageCount, _ := s.chatRepo.GetMessageCount(ctx, chatID)

	return &domain.ChatInfo{
		ID:           chat.ID,
		Name:         chat.Name,
		Description:  "",
		AvatarURL:    chat.AvatarURL,
		Type:         chat.Type,
		CreatedAt:    chat.CreatedAt,
		UpdatedAt:    chat.UpdatedAt,
		MemberCount:  memberCount,
		MessageCount: messageCount,
	}, nil
}

func (s *ChatInfoService) GetChatStats(ctx context.Context, chatID, userID string) (*domain.ChatStats, error) {
	isAdmin, _ := s.chatRepo.IsAdmin(ctx, chatID, userID)
	if !isAdmin {
		return nil, errors.New("only admin can view stats")
	}

	memberCount, _ := s.chatRepo.GetMemberCount(ctx, chatID)
	messageCount, _ := s.chatRepo.GetMessageCount(ctx, chatID)
	lastMsgTime, _ := s.chatRepo.GetLastMessageTime(ctx, chatID)

	return &domain.ChatStats{
		ChatID:        chatID,
		MemberCount:   memberCount,
		MessageCount:  messageCount,
		LastMessageAt: lastMsgTime,
	}, nil
}

func (s *ChatInfoService) LeaveChat(ctx context.Context, chatID, userID string) error {
	return s.chatRepo.RemoveParticipant(ctx, chatID, userID)
}

func (s *ChatInfoService) DeleteChat(ctx context.Context, chatID, userID string) error {
	chat, err := s.chatRepo.GetByID(ctx, chatID)
	if err != nil {
		return err
	}

	if chat.CreatedBy != userID {
		return errors.New("only chat owner can delete chat")
	}

	return s.chatRepo.Delete(ctx, chatID)
}
