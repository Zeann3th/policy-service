package unit

import (
	"context"
	"time"

	"github.com/zeann3th/policy-service/internal/core"
)

// Codes of the seeded units.
const (
	CodeBalance = "BALANCE"
	CodeAmount  = "AMOUNT"
	CodeCount   = "COUNT"
	CodePercent = "PERCENT"
)

type Unit struct {
	ID        int64
	Code      string
	Active    bool
	CreatedAt time.Time
	UpdatedAt *time.Time
}

type Response struct {
	ID        int64      `json:"id"`
	Code      string     `example:"AMOUNT" json:"code"`
	Active    bool       `json:"active"`
	CreatedAt time.Time  `format:"date-time" json:"createdAt"`
	UpdatedAt *time.Time `format:"date-time" json:"updatedAt,omitempty"`
}

type CreateRequest struct {
	Code      string `binding:"required" example:"AMOUNT" json:"code"`
	Active    *bool  `json:"active"`
	UpdatedBy string `binding:"required" example:"admin" json:"updatedBy"`
}

type UpdateRequest struct {
	Active    *bool  `binding:"required" json:"active"`
	UpdatedBy string `binding:"required" example:"admin" json:"updatedBy"`
}

type Store interface {
	GetActive(context.Context) ([]Unit, error)
	Create(context.Context, CreateRequest) (Unit, error)
	Get(context.Context, int64) (Unit, error)
	List(context.Context, int32, int32) ([]Unit, int64, error)
	Update(context.Context, int64, bool, string) (Unit, error)
	Delete(context.Context, int64, string) error
}

type Service interface {
	Create(context.Context, CreateRequest) (Response, error)
	Get(context.Context, int64) (Response, error)
	List(context.Context, core.PageRequest) (core.Page[Response], error)
	Update(context.Context, int64, UpdateRequest) (Response, error)
	Delete(context.Context, int64, core.ActorRequest) error
}
