package notification

import (
	"time"

	"github.com/google/uuid"
)

type Notification struct {
	EventID  uuid.UUID `json:"eventId"`
	Title    string    `json:"title"`
	StartsAt time.Time `json:"startsAt"`
	UserID   int64     `json:"userId"`
}
