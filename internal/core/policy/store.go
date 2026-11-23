package policy

import (
	"context"
	"time"
)

type Store interface {
	GetGroupID(ctx context.Context, walletType, currency, tier string) (int64, error)
	// GetPoliciesByGroupID returns the group's policies effective now.
	GetPoliciesByGroupID(ctx context.Context, groupID int64) ([]Policy, error)
	// GetNextPolicyStart returns when the group's next policy becomes
	// effective, or nil when none is scheduled.
	GetNextPolicyStart(ctx context.Context, groupID int64) (*time.Time, error)
	GetActiveCustomPolicies(ctx context.Context, walletID string) ([]CustomPolicy, error)

	CreateType(context.Context, CreatePolicyTypeRequest) (PolicyType, error)
	GetType(context.Context, int64) (PolicyType, error)
	ListTypes(context.Context, int32, int32) ([]PolicyType, int64, error)
	UpdateType(context.Context, int64, UpdatePolicyTypeRequest) (PolicyType, error)
	DeleteType(context.Context, int64, string) error

	CreateGroup(context.Context, CreatePolicyGroupRequest) (PolicyGroup, error)
	GetGroup(context.Context, int64) (PolicyGroup, error)
	ListGroups(context.Context, int32, int32) ([]PolicyGroup, int64, error)
	DeleteGroup(context.Context, int64, string) error

	CreatePolicy(context.Context, int64, CreatePolicyRequest) (Policy, error)
	GetPolicy(context.Context, int64, int64) (Policy, error)
	ListPolicies(context.Context, int64, int32, int32) ([]Policy, int64, error)
	UpdatePolicy(context.Context, int64, int64, UpdatePolicyRequest) (Policy, error)
	DeletePolicy(context.Context, int64, int64, string) error

	CreateCustomPolicy(context.Context, string, CreateCustomPolicyRequest) (CustomPolicy, error)
	GetCustomPolicy(context.Context, string, int64) (CustomPolicy, error)
	ListCustomPolicies(context.Context, string, int32, int32) ([]CustomPolicy, int64, error)
	UpdateCustomPolicy(context.Context, string, int64, UpdateCustomPolicyRequest) (CustomPolicy, error)
	DeleteCustomPolicy(context.Context, string, int64, string) error
}
