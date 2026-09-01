package repository

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"siakang-api/internal/modules/market/chat/domain"
	"siakang-api/pkg/logger"
)

// ErrThreadNotFound means "no such thread, OR the caller is not one of its two
// participants". The two are deliberately indistinguishable: the participation
// test lives in the WHERE clause of every query below, so a third party gets
// the same empty result as a bad id and the handler answers 404. A 403 would
// confirm the thread exists, which is itself the leak.
var ErrThreadNotFound = errors.New("chat thread not found")

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// participation is the whole authorization model for chat, expressed as a
// WHERE fragment against the thread's order: the caller is the order's
// customer, or the order's lapak. lapakID is a *string so a customer passes
// NULL — `o.lapak_id = NULL` is NULL, never true, so a customer can never
// match the lapak arm by accident.
const participation = `(o.customer_user_id = $1 OR o.lapak_id = $2)`

// LapakIDForUser returns the caller's lapak profile id, or "" when the caller
// is a customer. Chat needs it to evaluate the lapak arm of `participation`.
func (r *Repository) LapakIDForUser(ctx context.Context, userID string) (string, error) {
	const query = `SELECT id FROM market.lapak_profiles WHERE user_id = $1 AND deleted_at IS NULL`

	var id string
	err := r.db.QueryRow(ctx, query, userID).Scan(&id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", nil
		}
		logger.Error("Failed to resolve lapak profile", logger.Err(err))
		return "", err
	}
	return id, nil
}

// AssertParticipant is the gate in front of a single thread's history, its
// send path and its stream. It answers ErrThreadNotFound for a stranger and
// for a thread that does not exist — the same answer, on purpose.
//
// It is a separate query rather than folded into the reads below because a
// non-participant must get a 404, not an empty-but-successful page.
func (r *Repository) AssertParticipant(ctx context.Context, threadID, userID string, lapakID *string) error {
	const query = `
		SELECT 1
		FROM market.chat_threads t
		JOIN market.orders o ON o.id = t.order_id
		WHERE t.id = $3 AND ` + participation

	var one int
	err := r.db.QueryRow(ctx, query, userID, lapakID, threadID).Scan(&one)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrThreadNotFound
		}
		logger.Error("Failed to check chat thread participation", logger.Err(err))
		return err
	}
	return nil
}

// threadJoins resolves customer and lapak through the order, and the newest
// message through a LATERAL — one row per thread, no N+1, and the same
// subquery drives both last_message and the "most recently active" sort.
const threadJoins = `
	FROM market.chat_threads t
	JOIN market.orders o ON o.id = t.order_id
	JOIN core.users cu ON cu.id = o.customer_user_id
	JOIN market.lapak_profiles l ON l.id = o.lapak_id
	LEFT JOIN LATERAL (
		SELECT m.id, m.sender_user_id, m.body, m.created_at
		FROM market.chat_messages m
		WHERE m.thread_id = t.id
		ORDER BY m.created_at DESC, m.id DESC
		LIMIT 1
	) lm ON TRUE
`

// ListThreads returns one page of the caller's threads, most recently active
// first — newest message, falling back to the thread's own created_at when it
// has none.
//
// `, t.id DESC` is a tiebreak, not decoration: created_at defaults to NOW(),
// which in Postgres is the TRANSACTION timestamp, so rows written in one
// transaction share it byte-for-byte and their relative order would otherwise
// be undefined — a row could then appear on two LIMIT/OFFSET pages, or on none.
func (r *Repository) ListThreads(ctx context.Context, userID string, lapakID *string, page, limit int) ([]*domain.Thread, error) {
	const query = `
		SELECT t.id, t.order_id, t.created_at,
		       o.customer_user_id, COALESCE(cu.full_name, ''),
		       l.id, l.name, l.rating,
		       lm.id, lm.sender_user_id, lm.body, lm.created_at
		` + threadJoins + `
		WHERE ` + participation + `
		ORDER BY COALESCE(lm.created_at, t.created_at) DESC, t.id DESC
		LIMIT $3 OFFSET $4`

	rows, err := r.db.Query(ctx, query, userID, lapakID, limit, (page-1)*limit)
	if err != nil {
		logger.Error("Failed to list chat threads", logger.Err(err))
		return nil, err
	}
	defer rows.Close()

	threads := make([]*domain.Thread, 0)
	for rows.Next() {
		var t domain.Thread
		var msgID, senderID, body *string
		var msgAt *time.Time

		if err := rows.Scan(
			&t.ID, &t.OrderID, &t.CreatedAt,
			&t.Customer.ID, &t.Customer.FullName,
			&t.Lapak.ID, &t.Lapak.Name, &t.Lapak.Rating,
			&msgID, &senderID, &body, &msgAt,
		); err != nil {
			logger.Error("Failed to scan chat thread", logger.Err(err))
			return nil, err
		}

		if msgID != nil {
			t.LastMessage = &domain.Message{
				ID:           *msgID,
				ThreadID:     t.ID,
				SenderUserID: *senderID,
				Body:         *body,
				CreatedAt:    *msgAt,
			}
		}
		threads = append(threads, &t)
	}
	if err := rows.Err(); err != nil {
		logger.Error("Failed to iterate chat threads", logger.Err(err))
		return nil, err
	}
	return threads, nil
}

// CountThreads is the page total for ListThreads, over the same WHERE clause.
func (r *Repository) CountThreads(ctx context.Context, userID string, lapakID *string) (int64, error) {
	const query = `
		SELECT COUNT(*)
		FROM market.chat_threads t
		JOIN market.orders o ON o.id = t.order_id
		WHERE ` + participation

	var total int64
	if err := r.db.QueryRow(ctx, query, userID, lapakID).Scan(&total); err != nil {
		logger.Error("Failed to count chat threads", logger.Err(err))
		return 0, err
	}
	return total, nil
}

// ListMessages returns one page of a thread's history, newest first. The
// caller has already passed AssertParticipant.
func (r *Repository) ListMessages(ctx context.Context, threadID string, page, limit int) ([]domain.Message, error) {
	const query = `
		SELECT id, thread_id, sender_user_id, body, created_at
		FROM market.chat_messages
		WHERE thread_id = $1
		ORDER BY created_at DESC, id DESC
		LIMIT $2 OFFSET $3`

	rows, err := r.db.Query(ctx, query, threadID, limit, (page-1)*limit)
	if err != nil {
		logger.Error("Failed to list chat messages", logger.Err(err))
		return nil, err
	}
	defer rows.Close()

	messages := make([]domain.Message, 0)
	for rows.Next() {
		var m domain.Message
		if err := rows.Scan(&m.ID, &m.ThreadID, &m.SenderUserID, &m.Body, &m.CreatedAt); err != nil {
			logger.Error("Failed to scan chat message", logger.Err(err))
			return nil, err
		}
		messages = append(messages, m)
	}
	if err := rows.Err(); err != nil {
		logger.Error("Failed to iterate chat messages", logger.Err(err))
		return nil, err
	}
	return messages, nil
}

// CountMessages is the page total for ListMessages.
func (r *Repository) CountMessages(ctx context.Context, threadID string) (int64, error) {
	const query = `SELECT COUNT(*) FROM market.chat_messages WHERE thread_id = $1`

	var total int64
	if err := r.db.QueryRow(ctx, query, threadID).Scan(&total); err != nil {
		logger.Error("Failed to count chat messages", logger.Err(err))
		return 0, err
	}
	return total, nil
}

// InsertMessage writes one message and returns it as stored — id and
// created_at come back from the database rather than being guessed here, so
// the row the sender sees and the row the stream fans out are the same row.
func (r *Repository) InsertMessage(ctx context.Context, threadID, senderUserID, body string) (*domain.Message, error) {
	const query = `
		INSERT INTO market.chat_messages (thread_id, sender_user_id, body)
		VALUES ($1, $2, $3)
		RETURNING id, thread_id, sender_user_id, body, created_at`

	var m domain.Message
	err := r.db.QueryRow(ctx, query, threadID, senderUserID, body).
		Scan(&m.ID, &m.ThreadID, &m.SenderUserID, &m.Body, &m.CreatedAt)
	if err != nil {
		logger.Error("Failed to insert chat message", logger.Err(err))
		return nil, err
	}
	return &m, nil
}
