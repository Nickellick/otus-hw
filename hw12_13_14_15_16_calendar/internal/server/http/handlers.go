package internalhttp

import (
	"encoding/json"
	"errors"
	"io"
	"math"
	"net/http"
	"time"

	"github.com/Nickellick/otus-hw/hw12_13_14_15_16_calendar/internal/storage"
	"github.com/google/uuid"
)

type eventRequest struct {
	UserID              int64     `json:"userId"`
	Title               string    `json:"title"`
	Description         string    `json:"description"`
	StartsAt            time.Time `json:"startsAt"`
	EndsAt              time.Time `json:"endsAt"`
	NotifyBeforeSeconds int64     `json:"notifyBeforeSeconds"`
}

type createEventRequest struct {
	eventRequest
	ID uuid.UUID `json:"id"`
}

type createEventResponse struct {
	ID uuid.UUID `json:"id"`
}

func createEventHandler(app Application) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req createEventRequest

		if err := decodeJSON(r, &req); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}

		event, err := req.eventRequest.toEvent(req.ID)
		if err != nil {
			http.Error(w, "invalid event", http.StatusBadRequest)
			return
		}

		if err := app.CreateEvent(r.Context(), event); err != nil {
			writeEventError(w, err)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)

		_ = json.NewEncoder(w).Encode(createEventResponse{
			ID: event.ID,
		})
	}
}

func (req eventRequest) toEvent(id uuid.UUID) (storage.Event, error) {
	const maxSeconds = int64(math.MaxInt64) / int64(time.Second)

	if req.NotifyBeforeSeconds < 0 ||
		req.NotifyBeforeSeconds > maxSeconds {
		return storage.Event{}, storage.ErrInvalidEvent
	}

	return storage.Event{
		ID:           id,
		UserID:       req.UserID,
		Title:        req.Title,
		Description:  req.Description,
		StartsAt:     req.StartsAt,
		EndsAt:       req.EndsAt,
		NotifyBefore: time.Duration(req.NotifyBeforeSeconds) * time.Second,
	}, nil
}

func updateEventHandler(app Application) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := parseEventID(r)
		if err != nil || id == uuid.Nil {
			http.Error(w, "invalid event ID", http.StatusBadRequest)
			return
		}

		var req eventRequest
		if err := decodeJSON(r, &req); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}

		event, err := req.toEvent(id)
		if err != nil {
			http.Error(w, "invalid event", http.StatusBadRequest)
			return
		}

		if err := app.UpdateEvent(r.Context(), event); err != nil {
			writeEventError(w, err)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

func deleteEventHandler(app Application) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := parseEventID(r)
		if err != nil {
			http.Error(w, "invalid event ID", http.StatusBadRequest)
			return
		}

		if err := app.DeleteEvent(r.Context(), id); err != nil {
			writeEventError(w, err)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

func decodeJSON(r *http.Request, dst any) error {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(dst); err != nil {
		return err
	}

	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return errors.New("multiple JSON values")
		}
		return err
	}

	return nil
}

func parseEventID(r *http.Request) (uuid.UUID, error) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		return uuid.Nil, err
	}

	if id == uuid.Nil {
		return uuid.Nil, storage.ErrInvalidEvent
	}

	return id, nil
}

func writeEventError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, storage.ErrInvalidEvent):
		http.Error(w, "invalid event", http.StatusBadRequest)

	case errors.Is(err, storage.ErrEventNotFound):
		http.Error(w, "event not found", http.StatusNotFound)

	case errors.Is(err, storage.ErrEventExists),
		errors.Is(err, storage.ErrDateBusy):
		http.Error(w, "event conflict", http.StatusConflict)

	default:
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}
}
