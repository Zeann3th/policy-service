package rabbitmq

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/zeann3th/policy-service/internal/core/reservation"
)

const UsageReleasesQueue = "usage-releases"

func UsageReleaseHandler(service reservation.Service) Handler {
	return func(ctx context.Context, delivery amqp.Delivery) error {
		var event struct {
			RequestID string `json:"requestId"`
		}
		decoder := json.NewDecoder(bytes.NewReader(delivery.Body))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&event); err != nil {
			return fmt.Errorf("%w: decode usage release: %v", ErrReject, err)
		}
		if err := decoder.Decode(&struct{}{}); err != io.EOF {
			return fmt.Errorf("%w: usage release must contain one JSON object", ErrReject)
		}
		event.RequestID = strings.TrimSpace(event.RequestID)
		if event.RequestID == "" {
			return fmt.Errorf("%w: requestId is required", ErrReject)
		}
		_, err := service.Release(ctx, event.RequestID)
		return err
	}
}
