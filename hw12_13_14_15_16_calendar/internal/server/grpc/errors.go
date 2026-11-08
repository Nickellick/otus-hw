package internalgrpc

import (
	"errors"

	"github.com/Nickellick/otus-hw/hw12_13_14_15_16_calendar/internal/storage"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func toGRPCError(err error) error {
	switch {
	case errors.Is(err, storage.ErrInvalidEvent):
		return status.Error(codes.InvalidArgument, "invalid event")

	case errors.Is(err, storage.ErrEventNotFound):
		return status.Error(codes.NotFound, "event not found")

	case errors.Is(err, storage.ErrEventExists),
		errors.Is(err, storage.ErrDateBusy):
		return status.Error(codes.AlreadyExists, "event conflict")

	default:
		return status.Error(codes.Internal, "internal server error")
	}
}
