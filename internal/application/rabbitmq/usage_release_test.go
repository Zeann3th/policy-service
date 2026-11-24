package rabbitmq

import (
	"context"
	"errors"
	"testing"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/zeann3th/policy-service/internal/core/reservation"
)

func TestUsageReleaseHandlerReleasesRequest(t *testing.T) {
	service := &recordingReservationService{}
	handler := UsageReleaseHandler(service)

	if err := handler(context.Background(), amqp.Delivery{Body: []byte(`{"requestId":"req-1"}`)}); err != nil {
		t.Fatal(err)
	}
	if service.released != "req-1" {
		t.Fatalf("Release request ID = %q", service.released)
	}
}

func TestUsageReleaseHandlerRejectsInvalidPayload(t *testing.T) {
	for name, body := range map[string]string{
		"malformed JSON":   `{"requestId":`,
		"unknown field":    `{"requestId":"req-1","other":true}`,
		"blank request ID": `{"requestId":"  "}`,
		"trailing JSON":    `{"requestId":"req-1"}{}`,
	} {
		t.Run(name, func(t *testing.T) {
			err := UsageReleaseHandler(&recordingReservationService{})(context.Background(), amqp.Delivery{Body: []byte(body)})
			if !errors.Is(err, ErrReject) {
				t.Fatalf("handler error = %v, want ErrReject", err)
			}
		})
	}
}

func TestUsageReleaseHandlerReturnsServiceError(t *testing.T) {
	want := errors.New("release failed")
	service := &recordingReservationService{releaseErr: want}
	err := UsageReleaseHandler(service)(context.Background(), amqp.Delivery{Body: []byte(`{"requestId":"req-1"}`)})
	if !errors.Is(err, want) {
		t.Fatalf("handler error = %v, want %v", err, want)
	}
}

type recordingReservationService struct {
	released   string
	releaseErr error
}

func (s *recordingReservationService) Reserve(context.Context, reservation.ReserveRequest) (reservation.ReserveResponse, error) {
	return reservation.ReserveResponse{}, nil
}

func (s *recordingReservationService) Release(_ context.Context, requestID string) (reservation.ReleaseResponse, error) {
	s.released = requestID
	return reservation.ReleaseResponse{}, s.releaseErr
}
