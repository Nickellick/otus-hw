package app

import (
	"context"
	"time"

	"github.com/Nickellick/otus-hw/hw12_13_14_15_16_calendar/internal/storage"
	"github.com/google/uuid"
)

type Storage interface {
	Create(ctx context.Context, event storage.Event) error
	Update(ctx context.Context, event storage.Event) error
	Delete(ctx context.Context, id uuid.UUID) error

	List(
		ctx context.Context,
		userID int64,
		from time.Time,
		to time.Time,
	) ([]storage.Event, error)
}

type Logger interface {
	Info(msg string, args ...any)
	Error(msg string, args ...any)
}

type App struct {
	logger  Logger
	storage Storage
}

func New(logger Logger, storage Storage) *App {
	return &App{
		logger:  logger,
		storage: storage,
	}
}

func (a *App) CreateEvent(
	ctx context.Context,
	event storage.Event,
) error {
	if err := event.Validate(); err != nil {
		return err
	}

	return a.storage.Create(ctx, event)
}

func (a *App) UpdateEvent(
	ctx context.Context,
	event storage.Event,
) error {
	if err := event.Validate(); err != nil {
		return err
	}

	return a.storage.Update(ctx, event)
}

func (a *App) DeleteEvent(
	ctx context.Context,
	id uuid.UUID,
) error {
	return a.storage.Delete(ctx, id)
}

func (a *App) ListEvents(
	ctx context.Context,
	userID int64,
	from time.Time,
	to time.Time,
) ([]storage.Event, error) {
	return a.storage.List(ctx, userID, from, to)
}

func (a *App) ListEventsForDay(
	ctx context.Context,
	userID int64,
	date time.Time,
) ([]storage.Event, error) {
	start := time.Date(
		date.Year(), date.Month(), date.Day(),
		0, 0, 0, 0, date.Location(),
	)

	end := start.AddDate(0, 0, 1)

	return a.storage.List(ctx, userID, start, end)
}

func (a *App) ListEventsForWeek(
	ctx context.Context,
	userID int64,
	date time.Time,
) ([]storage.Event, error) {
	start := time.Date(
		date.Year(), date.Month(), date.Day(),
		0, 0, 0, 0, date.Location(),
	)

	daysSinceMonday := (int(start.Weekday()) + 6) % 7

	start = start.AddDate(0, 0, -daysSinceMonday)
	end := start.AddDate(0, 0, 7)

	return a.storage.List(ctx, userID, start, end)
}

func (a *App) ListEventsForMonth(
	ctx context.Context,
	userID int64,
	date time.Time,
) ([]storage.Event, error) {
	start := time.Date(
		date.Year(), date.Month(), 1,
		0, 0, 0, 0, date.Location(),
	)

	end := start.AddDate(0, 1, 0)

	return a.storage.List(ctx, userID, start, end)
}
