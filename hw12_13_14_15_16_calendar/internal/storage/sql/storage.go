package sqlstorage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/Nickellick/otus-hw/hw12_13_14_15_16_calendar/internal/outbox"
	"github.com/Nickellick/otus-hw/hw12_13_14_15_16_calendar/internal/storage"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	_ "github.com/jackc/pgx/v5/stdlib" //no:revive // https://pkg.go.dev/github.com/jackc/pgx/v5/stdlib
	"github.com/rgurov/pgerrors"
)

type Storage struct {
	db *sql.DB
}

func New(db *sql.DB) *Storage {
	return &Storage{db: db}
}

func Connect(ctx context.Context, dsn string) (*sql.DB, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return db, nil
}

func (s *Storage) Create(ctx context.Context, event storage.Event) error {
	if err := event.Validate(); err != nil {
		return err
	}

	const query = `
		INSERT INTO events (
			id,
			user_id,
			title,
			description,
			starts_at,
			ends_at,
			notify_before_ns
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`

	_, err := s.db.ExecContext(
		ctx,
		query,
		event.ID,
		event.UserID,
		event.Title,
		event.Description,
		event.StartsAt,
		event.EndsAt,
		int64(event.NotifyBefore),
	)
	if err != nil {
		return fmt.Errorf("create event: %w", mapError(err))
	}

	return nil
}

func (s *Storage) Update(ctx context.Context, event storage.Event) error {
	if err := event.Validate(); err != nil {
		return err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin update event transaction: %w", err)
	}
	defer tx.Rollback()

	const query = `
		UPDATE events
		SET
			user_id = $2,
			title = $3,
			description = $4,
			starts_at = $5,
			ends_at = $6,
			notify_before_ns = $7,
			notification_sent_at = CASE
				WHEN user_id IS DISTINCT FROM $2
				  OR title IS DISTINCT FROM $3
				  OR starts_at IS DISTINCT FROM $5
				  OR notify_before_ns IS DISTINCT FROM $7
				THEN NULL
				ELSE notification_sent_at
			END,
			version = version + CASE
				WHEN user_id IS DISTINCT FROM $2
				  OR title IS DISTINCT FROM $3
				  OR starts_at IS DISTINCT FROM $5
				  OR notify_before_ns IS DISTINCT FROM $7
				THEN 1
				ELSE 0
			END
		WHERE id = $1
		RETURNING version
	`

	var version int64

	err = tx.QueryRowContext(
		ctx,
		query,
		event.ID,
		event.UserID,
		event.Title,
		event.Description,
		event.StartsAt,
		event.EndsAt,
		int64(event.NotifyBefore),
	).Scan(&version)

	if errors.Is(err, sql.ErrNoRows) {
		return storage.ErrEventNotFound
	}
	if err != nil {
		return fmt.Errorf("update event: %w", mapError(err))
	}

	const cancelOutbox = `
		UPDATE notification_outbox
		SET
			status = 'cancelled',
			claim_id = NULL,
			claimed_until = NULL
		WHERE event_id = $1
		  AND event_version < $2
		  AND status IN ('pending', 'processing')
	`

	if _, err := tx.ExecContext(
		ctx,
		cancelOutbox,
		event.ID,
		version,
	); err != nil {
		return fmt.Errorf("cancel stale notifications: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit event update: %w", err)
	}

	return nil
}

func (s *Storage) Delete(ctx context.Context, id uuid.UUID) error {
	const query = `DELETE FROM events WHERE id = $1`

	result, err := s.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete event: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("get deleted rows count: %w", err)
	}

	if affected == 0 {
		return storage.ErrEventNotFound
	}

	return nil
}

func (s *Storage) List(
	ctx context.Context,
	userID int64,
	from time.Time,
	to time.Time,
) ([]storage.Event, error) {
	if !to.After(from) {
		return nil, fmt.Errorf(
			"%w: invalid search interval",
			storage.ErrInvalidEvent,
		)
	}

	const query = `
		SELECT
			id,
			user_id,
			title,
			description,
			starts_at,
			ends_at,
			notify_before_ns
		FROM events
		WHERE user_id = $1
		  AND starts_at < $3
		  AND ends_at > $2
		ORDER BY starts_at, id
	`

	rows, err := s.db.QueryContext(ctx, query, userID, from, to)
	if err != nil {
		return nil, fmt.Errorf("list events: %w", err)
	}
	defer rows.Close()

	events := make([]storage.Event, 0)

	for rows.Next() {
		var event storage.Event
		var notifyBeforeNS int64

		err := rows.Scan(
			&event.ID,
			&event.UserID,
			&event.Title,
			&event.Description,
			&event.StartsAt,
			&event.EndsAt,
			&notifyBeforeNS,
		)
		if err != nil {
			return nil, fmt.Errorf("scan event: %w", err)
		}

		event.NotifyBefore = time.Duration(notifyBeforeNS)

		events = append(events, event)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate events: %w", err)
	}

	return events, nil
}

func (s *Storage) DueEvents(
	ctx context.Context,
	now time.Time,
	limit int,
) ([]storage.Event, error) {
	const query = `
		SELECT
			id,
			user_id,
			title,
			description,
			starts_at,
			ends_at,
			notify_before_ns
		FROM events
		WHERE notify_before_ns > 0
		  AND notification_sent_at IS NULL
		  AND starts_at - (
		      (notify_before_ns / 1000) * INTERVAL '1 microsecond'
		  ) <= $1
		  AND starts_at > $1
		ORDER BY starts_at, id
		LIMIT $2
	`

	if limit <= 0 {
		return nil, fmt.Errorf("limit must be positive")
	}

	rows, err := s.db.QueryContext(ctx, query, now, limit)
	if err != nil {
		return nil, fmt.Errorf("query due events: %w", err)
	}
	defer rows.Close()

	events := make([]storage.Event, 0)

	for rows.Next() {
		var event storage.Event
		var notifyBeforeNS int64

		if err := rows.Scan(
			&event.ID,
			&event.UserID,
			&event.Title,
			&event.Description,
			&event.StartsAt,
			&event.EndsAt,
			&notifyBeforeNS,
		); err != nil {
			return nil, fmt.Errorf("scan due event: %w", err)
		}

		event.NotifyBefore = time.Duration(notifyBeforeNS)
		events = append(events, event)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate due events: %w", err)
	}

	return events, nil
}

func (s *Storage) MarkNotified(
	ctx context.Context,
	id uuid.UUID,
	startsAt time.Time,
	notifyBefore time.Duration,
) (bool, error) {
	const query = `
		UPDATE events
		SET notification_sent_at = NOW()
		WHERE id = $1
		  AND starts_at = $2
		  AND notify_before_ns = $3
		  AND notification_sent_at IS NULL
	`

	result, err := s.db.ExecContext(
		ctx,
		query,
		id,
		startsAt,
		int64(notifyBefore),
	)
	if err != nil {
		return false, fmt.Errorf("mark event notified: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("rows affected: %w", err)
	}

	return affected == 1, nil
}

func (s *Storage) DeleteOldEvents(
	ctx context.Context,
	before time.Time,
) (int64, error) {
	const query = `
		DELETE FROM events
		WHERE ends_at < $1
	`

	result, err := s.db.ExecContext(ctx, query, before)
	if err != nil {
		return 0, fmt.Errorf("delete old events: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("rows affected: %w", err)
	}

	return affected, nil
}

func (s *Storage) EnqueueDue(
	ctx context.Context,
	now time.Time,
	limit int,
) (int64, error) {
	if limit <= 0 {
		return 0, fmt.Errorf("limit must be positive")
	}

	const query = `
		WITH due AS (
			SELECT
				id,
				version,
				user_id,
				title,
				starts_at
			FROM events
			WHERE notify_before_ns > 0
			  AND notification_sent_at IS NULL
			  AND starts_at - (
			      (notify_before_ns / 1000) * INTERVAL '1 microsecond'
			  ) <= $1
			  AND starts_at > $1
			ORDER BY starts_at, id
			LIMIT $2
			FOR UPDATE SKIP LOCKED
		)
		INSERT INTO notification_outbox (
			id,
			event_id,
			event_version,
			payload
		)
		SELECT
			gen_random_uuid(),
			id,
			version,
			jsonb_build_object(
				'eventId', id,
				'eventVersion', version,
				'title', title,
				'startsAt', starts_at,
				'userId', user_id
			)
		FROM due
		ON CONFLICT (event_id, event_version)
		DO NOTHING
	`

	result, err := s.db.ExecContext(ctx, query, now, limit)
	if err != nil {
		return 0, fmt.Errorf("enqueue due notifications: %w", err)
	}

	count, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("count enqueued notifications: %w", err)
	}

	return count, nil
}

func (s *Storage) Claim(
	ctx context.Context,
	limit int,
	lease time.Duration,
) ([]outbox.ClaimedMessage, error) {
	if limit <= 0 || lease <= 0 {
		return nil, fmt.Errorf("invalid claim parameters")
	}

	const query = `
		WITH candidates AS (
			SELECT id
			FROM notification_outbox
			WHERE status = 'pending'
			   OR (
			       status = 'processing'
			       AND claimed_until < NOW()
			   )
			ORDER BY created_at, id
			LIMIT $1
			FOR UPDATE SKIP LOCKED
		)
		UPDATE notification_outbox o
		SET
			status = 'processing',
			claim_id = gen_random_uuid(),
			claimed_until = NOW() + (
				$2::BIGINT * INTERVAL '1 microsecond'
			)
		FROM candidates c
		WHERE o.id = c.id
		RETURNING
			o.id,
			o.event_id,
			o.event_version,
			o.payload,
			o.claim_id
	`

	rows, err := s.db.QueryContext(
		ctx,
		query,
		limit,
		lease.Microseconds(),
	)
	if err != nil {
		return nil, fmt.Errorf("claim outbox: %w", err)
	}
	defer rows.Close()

	messages := make([]outbox.ClaimedMessage, 0)

	for rows.Next() {
		var msg outbox.ClaimedMessage

		err := rows.Scan(
			&msg.ID,
			&msg.EventID,
			&msg.EventVersion,
			&msg.Payload,
			&msg.ClaimID,
		)
		if err != nil {
			return nil, fmt.Errorf("scan outbox message: %w", err)
		}

		messages = append(messages, msg)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate outbox: %w", err)
	}

	return messages, nil
}

func (s *Storage) IsCurrent(
	ctx context.Context,
	msg outbox.ClaimedMessage,
) (bool, error) {
	const query = `
		SELECT EXISTS (
			SELECT 1
			FROM notification_outbox o
			JOIN events e ON e.id = o.event_id
			WHERE o.id = $1
			  AND o.claim_id = $2
			  AND o.status = 'processing'
			  AND o.claimed_until > NOW()
			  AND e.version = o.event_version
			  AND e.notification_sent_at IS NULL
			  AND e.notify_before_ns > 0
			  AND e.starts_at > NOW()
		)
	`

	var current bool

	if err := s.db.QueryRowContext(
		ctx,
		query,
		msg.ID,
		msg.ClaimID,
	).Scan(&current); err != nil {
		return false, fmt.Errorf("check outbox version: %w", err)
	}

	return current, nil
}

func (s *Storage) MarkSent(
	ctx context.Context,
	msg outbox.ClaimedMessage,
) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin mark sent transaction: %w", err)
	}
	defer tx.Rollback()

	// Сначала блокируем событие.
	// Update() захватывает блокировки в таком же порядке.
	var version int64

	err = tx.QueryRowContext(
		ctx,
		`SELECT version FROM events WHERE id = $1 FOR UPDATE`,
		msg.EventID,
	).Scan(&version)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("event %s no longer exists", msg.EventID)
		}

		return fmt.Errorf("lock event: %w", err)
	}

	// Затем обновляем outbox-запись, только если
	// всё ещё владеем этой попыткой обработки.
	const markOutbox = `
		UPDATE notification_outbox
		SET
			status = 'sent',
			sent_at = NOW(),
			claimed_until = NULL,
			claim_id = NULL
		WHERE id = $1
		  AND claim_id = $2
		  AND status = 'processing'
		  AND claimed_until > NOW()
	`

	result, err := tx.ExecContext(
		ctx,
		markOutbox,
		msg.ID,
		msg.ClaimID,
	)
	if err != nil {
		return fmt.Errorf("mark outbox sent: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("count marked outbox rows: %w", err)
	}
	if affected != 1 {
		return fmt.Errorf("outbox claim no longer owned")
	}

	// Отмечаем событие только при совпадении версии.
	if version == msg.EventVersion {
		const markEvent = `
			UPDATE events
			SET notification_sent_at = NOW()
			WHERE id = $1
			  AND version = $2
			  AND notification_sent_at IS NULL
		`

		if _, err := tx.ExecContext(
			ctx,
			markEvent,
			msg.EventID,
			msg.EventVersion,
		); err != nil {
			return fmt.Errorf("mark event notified: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit mark sent: %w", err)
	}

	return nil
}

func (s *Storage) Cancel(
	ctx context.Context,
	msg outbox.ClaimedMessage,
) error {
	const query = `
		UPDATE notification_outbox
		SET
			status = 'cancelled',
			claim_id = NULL,
			claimed_until = NULL
		WHERE id = $1
		  AND claim_id = $2
		  AND status = 'processing'
	`

	result, err := s.db.ExecContext(
		ctx,
		query,
		msg.ID,
		msg.ClaimID,
	)
	if err != nil {
		return fmt.Errorf("cancel outbox message: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("count cancelled messages: %w", err)
	}
	if affected != 1 {
		return fmt.Errorf("outbox claim no longer owned")
	}

	return nil
}

func (s *Storage) IsNotificationActual(
	ctx context.Context,
	eventID uuid.UUID,
	version int64,
) (bool, error) {
	const query = `
        SELECT EXISTS (
            SELECT 1
            FROM events
            WHERE id = $1
              AND version = $2
        )
    `

	var actual bool

	err := s.db.QueryRowContext(
		ctx,
		query,
		eventID,
		version,
	).Scan(&actual)
	if err != nil {
		return false, fmt.Errorf("check notification actuality: %w", err)
	}

	return actual, nil
}

func mapError(err error) error {
	if err == nil {
		return nil
	}

	var pgErr *pgconn.PgError

	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case pgerrors.ExclusionViolation:
			return fmt.Errorf("%w: %w", storage.ErrDateBusy, err)

		case pgerrors.UniqueViolation:
			return fmt.Errorf("%w: %w", storage.ErrEventExists, err)

		case pgerrors.CheckViolation:
			return fmt.Errorf("%w: %w", storage.ErrInvalidEvent, err)
		}
	}

	return err
}
