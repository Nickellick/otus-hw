package internalgrpc

import (
	"context"
	"time"

	"github.com/Nickellick/otus-hw/hw12_13_14_15_16_calendar/internal/storage"
	pb "github.com/Nickellick/otus-hw/hw12_13_14_15_16_calendar/pkg/calendarpb"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func eventToProto(event storage.Event) *pb.Event {
	return &pb.Event{
		Id:           event.ID.String(),
		UserId:       event.UserID,
		Title:        event.Title,
		Description:  event.Description,
		StartsAt:     timestamppb.New(event.StartsAt),
		EndsAt:       timestamppb.New(event.EndsAt),
		NotifyBefore: durationpb.New(event.NotifyBefore),
	}
}

func eventFromProto(event *pb.Event) (storage.Event, error) {
	if event == nil {
		return storage.Event{}, status.Error(
			codes.InvalidArgument, "event is required",
		)
	}

	id, err := uuid.Parse(event.GetId())
	if err != nil || id == uuid.Nil {
		return storage.Event{}, status.Error(
			codes.InvalidArgument, "invalid event ID",
		)
	}

	startsAt := event.GetStartsAt()
	endsAt := event.GetEndsAt()

	if startsAt == nil || endsAt == nil ||
		!startsAt.IsValid() || !endsAt.IsValid() {
		return storage.Event{}, status.Error(
			codes.InvalidArgument, "invalid event timestamps",
		)
	}

	notifyBefore := event.GetNotifyBefore()
	if notifyBefore != nil && !notifyBefore.IsValid() {
		return storage.Event{}, status.Error(
			codes.InvalidArgument, "invalid notification duration",
		)
	}

	result := storage.Event{
		ID:          id,
		UserID:      event.GetUserId(),
		Title:       event.GetTitle(),
		Description: event.GetDescription(),
		StartsAt:    startsAt.AsTime(),
		EndsAt:      endsAt.AsTime(),
	}

	if notifyBefore != nil {
		result.NotifyBefore = notifyBefore.AsDuration()
	}

	if err := result.Validate(); err != nil {
		return storage.Event{}, toGRPCError(err)
	}

	return result, nil
}

func (s *Server) CreateEvent(
	ctx context.Context,
	req *pb.CreateEventRequest,
) (*pb.CreateEventResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}

	event, err := eventFromProto(req.GetEvent())
	if err != nil {
		return nil, err
	}

	if err := s.app.CreateEvent(ctx, event); err != nil {
		return nil, toGRPCError(err)
	}

	return &pb.CreateEventResponse{
		Id: event.ID.String(),
	}, nil
}

func (s *Server) UpdateEvent(
	ctx context.Context,
	req *pb.UpdateEventRequest,
) (*pb.UpdateEventResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}

	event, err := eventFromProto(req.GetEvent())
	if err != nil {
		return nil, err
	}

	if err := s.app.UpdateEvent(ctx, event); err != nil {
		return nil, toGRPCError(err)
	}

	return &pb.UpdateEventResponse{}, nil
}

func (s *Server) DeleteEvent(
	ctx context.Context,
	req *pb.DeleteEventRequest,
) (*pb.DeleteEventResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}

	id, err := uuid.Parse(req.GetId())
	if err != nil || id == uuid.Nil {
		return nil, status.Error(codes.InvalidArgument, "invalid event ID")
	}

	if err := s.app.DeleteEvent(ctx, id); err != nil {
		return nil, toGRPCError(err)
	}

	return &pb.DeleteEventResponse{}, nil
}

type listEventsFunc func(
	context.Context,
	int64,
	time.Time,
) ([]storage.Event, error)

func (s *Server) listEvents(
	ctx context.Context,
	req *pb.ListEventsRequest,
	list listEventsFunc,
) (*pb.ListEventsResponse, error) {
	if req == nil {
		return nil, status.Error(
			codes.InvalidArgument,
			"request is required",
		)
	}

	if req.GetUserId() <= 0 {
		return nil, status.Error(
			codes.InvalidArgument,
			"invalid user ID",
		)
	}

	date := req.GetDate()
	if date == nil || !date.IsValid() {
		return nil, status.Error(
			codes.InvalidArgument,
			"invalid date",
		)
	}

	events, err := list(
		ctx,
		req.GetUserId(),
		date.AsTime().UTC(),
	)
	if err != nil {
		return nil, toGRPCError(err)
	}

	response := &pb.ListEventsResponse{
		Events: make([]*pb.Event, 0, len(events)),
	}

	for _, event := range events {
		response.Events = append(
			response.Events,
			eventToProto(event),
		)
	}

	return response, nil
}

func (s *Server) ListEventsForDay(
	ctx context.Context,
	req *pb.ListEventsRequest,
) (*pb.ListEventsResponse, error) {
	return s.listEvents(
		ctx,
		req,
		s.app.ListEventsForDay,
	)
}

func (s *Server) ListEventsForWeek(
	ctx context.Context,
	req *pb.ListEventsRequest,
) (*pb.ListEventsResponse, error) {
	return s.listEvents(
		ctx,
		req,
		s.app.ListEventsForWeek,
	)
}

func (s *Server) ListEventsForMonth(
	ctx context.Context,
	req *pb.ListEventsRequest,
) (*pb.ListEventsResponse, error) {
	return s.listEvents(
		ctx,
		req,
		s.app.ListEventsForMonth,
	)
}
