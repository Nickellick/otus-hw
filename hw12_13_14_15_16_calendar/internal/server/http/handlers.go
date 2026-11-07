package internalhttp

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/Nickellick/otus-hw/hw12_13_14_15_16_calendar/internal/storage"
	"github.com/google/uuid"
)

type createEventRequest struct {
	ID                  uuid.UUID `json:"id"`
	UserID              int64     `json:"userId"`
	Title               string    `json:"title"`
	Description         string    `json:"description"`
	StartsAt            time.Time `json:"startsAt"`
	EndsAt              time.Time `json:"endsAt"`
	NotifyBeforeSeconds int64     `json:"notifyBeforeSeconds"`
}

type createEventResponse struct {
	ID uuid.UUID `json:"id"`
}

func createEventHandler(app Application) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req createEventRequest

		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()

		if err := decoder.Decode(&req); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}

		var extra any
		if err := decoder.Decode(&extra); err != io.EOF {
			http.Error(w, "unexpected data after JSON", http.StatusBadRequest)
			return
		}

		event := storage.Event{
			ID:           req.ID,
			UserID:       req.UserID,
			Title:        req.Title,
			Description:  req.Description,
			StartsAt:     req.StartsAt,
			EndsAt:       req.EndsAt,
			NotifyBefore: time.Duration(req.NotifyBeforeSeconds) * time.Second,
		}

		if err := app.CreateEvent(r.Context(), event); err != nil {
			switch {
			case errors.Is(err, storage.ErrInvalidEvent):
				http.Error(w, err.Error(), http.StatusBadRequest)
			case errors.Is(err, storage.ErrEventExists),
				errors.Is(err, storage.ErrDateBusy):
				http.Error(w, err.Error(), http.StatusConflict)
			default:
				http.Error(w, "internal server error", http.StatusInternalServerError)
			}
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)

		_ = json.NewEncoder(w).Encode(createEventResponse{
			ID: event.ID,
		})
	}
}
