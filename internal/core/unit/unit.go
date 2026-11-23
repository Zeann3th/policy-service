// Package unit manages the units catalog: what a policy measures. The
// service enforces the units it has a measure for (policy.UnitAmount,
// policy.UnitCount); any other unit, such as BALANCE, is stored and served
// for another service to enforce.
package unit

import (
	"context"
	"time"

	"github.com/zeann3th/policy-service/internal/core"
)

type Unit struct {
	ID          int64
	Code        string
	Description *string
	Active      bool
	CreatedAt   time.Time
	UpdatedAt   *time.Time
}

type Response struct {
	ID          int64      `json:"id"`
	Code        string     `example:"AMOUNT" json:"code"`
	Description *string    `json:"description,omitempty"`
	Active      bool       `json:"active"`
	CreatedAt   time.Time  `format:"date-time" json:"createdAt"`
	UpdatedAt   *time.Time `format:"date-time" json:"updatedAt,omitempty"`
}

type CreateRequest struct {
	Code        string  `binding:"required" example:"AMOUNT" json:"code"`
	Description *string `json:"description"`
	Active      *bool   `json:"active"`
	UpdatedBy   string  `binding:"required" example:"admin" json:"updatedBy"`
}

type UpdateRequest struct {
	Description *string `json:"description"`
	Active      *bool   `binding:"required" json:"active"`
	UpdatedBy   string  `binding:"required" example:"admin" json:"updatedBy"`
}

type Store interface {
	Create(context.Context, CreateRequest) (Unit, error)
	Get(context.Context, int64) (Unit, error)
	List(context.Context, int32, int32) ([]Unit, int64, error)
	Update(context.Context, int64, UpdateRequest) (Unit, error)
	Delete(context.Context, int64, string) error
}

type Service interface {
	Create(context.Context, CreateRequest) (Response, error)
	Get(context.Context, int64) (Response, error)
	List(context.Context, core.PageRequest) (core.Page[Response], error)
	Update(context.Context, int64, UpdateRequest) (Response, error)
	Delete(context.Context, int64, core.ActorRequest) error
}
