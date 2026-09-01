// Package service holds the chat business logic: the participation gate in
// front of every thread, and the in-process hub that fans a stored message out
// to the streams open on its thread.
//
// It exists here where wallet and me have no service layer because the stream
// has a lifecycle — subscribe, heartbeat, expire, unsubscribe — and that is
// not a handler's job.
package service

import (
	"context"

	"siakang-api/internal/modules/market/chat/domain"
	"siakang-api/internal/modules/market/chat/repository"
)

type Service struct {
	repo *repository.Repository
	hub  *Hub
}

func NewService(repo *repository.Repository) *Service {
	return &Service{repo: repo, hub: NewHub()}
}

// Hub exposes the hub so the handler can report subscriber counts in its
// stream open/close logs — the first thing anyone reads when QA says "the
// message did not arrive".
func (s *Service) Hub() *Hub { return s.hub }

// ListThreads returns one page of the caller's threads plus the total.
func (s *Service) ListThreads(ctx context.Context, userID string, page, limit int) ([]*domain.Thread, int64, error) {
	lapakID, err := s.lapakID(ctx, userID)
	if err != nil {
		return nil, 0, err
	}

	total, err := s.repo.CountThreads(ctx, userID, lapakID)
	if err != nil {
		return nil, 0, err
	}

	threads, err := s.repo.ListThreads(ctx, userID, lapakID, page, limit)
	if err != nil {
		return nil, 0, err
	}
	return threads, total, nil
}

// ListMessages returns one page of a thread's history, newest first, or
// repository.ErrThreadNotFound when the caller is not a participant.
func (s *Service) ListMessages(ctx context.Context, userID, threadID string, page, limit int) ([]domain.Message, int64, error) {
	if err := s.assert(ctx, userID, threadID); err != nil {
		return nil, 0, err
	}

	total, err := s.repo.CountMessages(ctx, threadID)
	if err != nil {
		return nil, 0, err
	}

	messages, err := s.repo.ListMessages(ctx, threadID, page, limit)
	if err != nil {
		return nil, 0, err
	}
	return messages, total, nil
}

// Send persists a message and fans it out to every stream open on the thread,
// the sender's included. The fan-out is after the insert on purpose: nothing
// reaches a stream that is not already durable, so a client never renders a
// message that a failed write means never existed.
func (s *Service) Send(ctx context.Context, userID, threadID, body string) (*domain.Message, error) {
	if err := s.assert(ctx, userID, threadID); err != nil {
		return nil, err
	}

	msg, err := s.repo.InsertMessage(ctx, threadID, userID, body)
	if err != nil {
		return nil, err
	}

	s.hub.Publish(threadID, *msg)
	return msg, nil
}

// Subscribe gates a stream on participation and registers it with the hub. It
// returns the channel and the unsubscribe the handler must defer — the two
// are handed back together so there is no way to take one without the other.
func (s *Service) Subscribe(ctx context.Context, userID, threadID string) (<-chan domain.Message, func() int, error) {
	if err := s.assert(ctx, userID, threadID); err != nil {
		return nil, nil, err
	}

	ch := s.hub.Subscribe(threadID)
	return ch, func() int { return s.hub.Unsubscribe(threadID, ch) }, nil
}

func (s *Service) assert(ctx context.Context, userID, threadID string) error {
	lapakID, err := s.lapakID(ctx, userID)
	if err != nil {
		return err
	}
	return s.repo.AssertParticipant(ctx, threadID, userID, lapakID)
}

// lapakID resolves the caller's lapak profile, as a *string so that a customer
// passes SQL NULL. That is what keeps `o.lapak_id = $2` from ever matching for
// someone who has no profile.
func (s *Service) lapakID(ctx context.Context, userID string) (*string, error) {
	id, err := s.repo.LapakIDForUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	if id == "" {
		return nil, nil
	}
	return &id, nil
}
