// Mirrors the policy service's HTTP DTOs (docs/swagger.yaml). Money and
// policy values travel as decimal strings and are never parsed to numbers.

export type Kind = "LIMIT" | "WAIVER" | "FEE" | "DISCOUNT" | "TAX";
export type Unit = "AMOUNT" | "COUNT" | "BALANCE";
export type Base = "AMOUNT" | "FEE" | "NET_FEE";
export type Operator = "EQ" | "NEQ" | "GT" | "GTE" | "LT" | "LTE";
export type Direction = "ALL" | "OUT" | "IN";
export type WindowType = "NONE" | "DAILY" | "MONTHLY" | "LIFETIME";
export type Source = "SYSTEM" | "USER";
export type Status = "RESERVED" | "REJECTED";

export interface Page<T> {
  items: T[];
  page: number;
  limit: number;
  total: number;
}

export interface ApiError {
  code: string;
  message: string;
}

export interface PolicyType {
  id: number;
  code: string;
  description?: string;
  kind: Kind;
  active: boolean;
  createdAt: string;
  updatedAt?: string;
}

export interface PolicyGroup {
  id: number;
  walletType: string;
  currency: string;
  tier: string;
  createdAt: string;
  updatedAt?: string;
}

// Value fields shared by system and custom policies.
export interface PolicyValue {
  policyTypeId: number;
  transactionType: string;
  channel: string;
  direction: Direction;
  windowType: WindowType;
  unit?: Unit;
  base?: Base;
  operator?: Operator;
  value: string;
  effectiveFrom: string;
  effectiveTo?: string;
}

export interface Policy extends PolicyValue {
  id: number;
  policyGroupId: number;
  createdAt: string;
  updatedAt?: string;
}

export interface CustomPolicy extends PolicyValue {
  id: number;
  walletId: string;
  reason?: string;
  createdBy: string;
  createdAt: string;
  updatedAt?: string;
}

export interface Rule {
  policyType: string;
  kind: Kind;
  transactionType: string;
  channel: string;
  direction: Direction;
  windowType: WindowType;
  unit?: Unit;
  base?: Base;
  operator?: Operator;
  value: string;
  source: Source;
}

export interface Party {
  walletId: string;
  walletType: string;
  tier: string;
}

export interface AppliedPolicy {
  policyId: number;
  policyType: string;
  kind: Kind;
  unit?: Unit;
  base?: Base;
  source: Source;
  transactionType: string;
  channel: string;
  direction: Direction;
  windowType: WindowType;
  windowStart?: string;
  operator?: Operator;
  value: string;
  usage?: string;
  /** What this FEE / DISCOUNT / TAX line computed, before caps and waiver. */
  charge?: string;
  violated: boolean;
}

export interface DryRunUsage {
  policyId: number;
  windowStart: string;
  live: string;
  used: string;
  overridden: boolean;
}

export interface DryRunSide {
  walletId: string;
  isViolated: boolean;
  fee: string;
  discount: string;
  tax: string;
  appliedPolicies: AppliedPolicy[];
  usage: DryRunUsage[];
}

export interface DryRunRequest {
  transactionType: string;
  channel: string;
  currency: string;
  amount: string;
  sender: Party;
  receiver: Party;
  usageOverrides: { sender?: Record<string, string>; receiver?: Record<string, string> };
}

export interface DryRunResponse {
  status: Status;
  sender: DryRunSide;
  receiver: DryRunSide;
  evaluatedAt: string;
}

export interface ReloadResponse {
  evicted: boolean;
  reloadedNamespaces?: string[];
}
