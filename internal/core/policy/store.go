package policy

import (
	"context"
	"time"
)

// The persistence ports, one per use, so each service depends only on what
// it calls. Store composes them for an adapter that implements them all.

// ResolveStore reads what a wallet's policies are resolved from.
type ResolveStore interface {
	GetGroupID(ctx context.Context, walletType, currency, tier string) (int64, error)
	// GetPoliciesByGroupID returns the group's policies effective now.
	GetPoliciesByGroupID(ctx context.Context, groupID int64) ([]Policy, error)
	// GetNextPolicyStart returns when the group's next policy becomes
	// effective, or nil when none is scheduled.
	GetNextPolicyStart(ctx context.Context, groupID int64) (*time.Time, error)
	GetActiveCustomPolicies(ctx context.Context, walletID string) ([]CustomPolicy, error)
}

// CatalogReader resolves the catalog rows a policy refers to.
type CatalogReader interface {
	GetType(context.Context, int64) (PolicyType, error)
	// GetUnitCode returns the code of a unit in the units catalog.
	GetUnitCode(ctx context.Context, id int64) (string, error)
}

type TypeStore interface {
	CreateType(context.Context, CreatePolicyTypeRequest) (PolicyType, error)
	GetType(context.Context, int64) (PolicyType, error)
	ListTypes(context.Context, int32, int32) ([]PolicyType, int64, error)
	UpdateType(context.Context, int64, UpdatePolicyTypeRequest) (PolicyType, error)
	DeleteType(context.Context, int64, string) error
}

type GroupStore interface {
	CreateGroup(context.Context, CreatePolicyGroupRequest) (PolicyGroup, error)
	GetGroup(context.Context, int64) (PolicyGroup, error)
	ListGroups(context.Context, int32, int32) ([]PolicyGroup, int64, error)
	DeleteGroup(context.Context, int64, string) error
}

// SystemPolicyStore manages a group's policies.
type SystemPolicyStore interface {
	CreatePolicy(context.Context, int64, CreatePolicyRequest) (Policy, error)
	GetPolicy(context.Context, int64, int64) (Policy, error)
	ListPolicies(context.Context, int64, int32, int32) ([]Policy, int64, error)
	UpdatePolicy(context.Context, int64, int64, UpdatePolicyRequest) (Policy, error)
	DeletePolicy(context.Context, int64, int64, string) error
}

// CustomPolicyStore manages a wallet's custom policies.
type CustomPolicyStore interface {
	CreateCustomPolicy(context.Context, string, CreateCustomPolicyRequest) (CustomPolicy, error)
	GetCustomPolicy(context.Context, string, int64) (CustomPolicy, error)
	ListCustomPolicies(context.Context, string, int32, int32) ([]CustomPolicy, int64, error)
	UpdateCustomPolicy(context.Context, string, int64, UpdateCustomPolicyRequest) (CustomPolicy, error)
	DeleteCustomPolicy(context.Context, string, int64, string) error
}

// PolicyStore is what the policy service needs: resolving, the catalog it
// validates against, and both kinds of policies.
type PolicyStore interface {
	ResolveStore
	CatalogReader
	SystemPolicyStore
	CustomPolicyStore
}

// Store is every port; one adapter implements it.
type Store interface {
	PolicyStore
	TypeStore
	GroupStore
}
