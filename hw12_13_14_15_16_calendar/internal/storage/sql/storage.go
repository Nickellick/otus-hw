package sqlstorage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

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

	const query = `
		UPDATE events
		SET
			user_id = $2,
			title = $3,
			description = $4,
			starts_at = $5,
			ends_at = $6,
			notify_before_ns = $7
		WHERE id = $1
	`

	result, err := s.db.ExecContext(
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
		return fmt.Errorf("update event: %w", mapError(err))
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("get updated rows count: %w", err)
	}

	if affected == 0 {
		return storage.ErrEventNotFound
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
