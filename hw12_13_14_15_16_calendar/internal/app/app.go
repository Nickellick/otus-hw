package app

import (
	"context"
	"time"

	"github.com/google/uuid"

	// Замени на актуальный путь из go.mod.
	"github.com/Nickellick/otus-hw/hw12_13_14_15_16_calendar/internal/storage"
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
	Info(msg string)
	Error(msg string)
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
