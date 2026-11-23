import type { Base, Kind, Operator, PolicyValue, Unit, WindowType } from "./types";

// Which value fields each kind takes. Mirrors policy.shapes in the service,
// which stays the authority: it rejects anything that does not fit.
export interface Shape {
  units: Unit[];
  bases: Base[];
  operator: boolean;
  windowed: boolean;
}

export const SHAPES: Record<Kind, Shape> = {
  LIMIT: { units: ["AMOUNT", "COUNT", "BALANCE"], bases: [], operator: true, windowed: true },
  WAIVER: { units: ["COUNT"], bases: [], operator: false, windowed: true },
  FEE: { units: [], bases: ["AMOUNT"], operator: false, windowed: false },
  DISCOUNT: { units: [], bases: ["AMOUNT", "FEE"], operator: false, windowed: false },
  TAX: { units: [], bases: ["AMOUNT", "FEE", "NET_FEE"], operator: false, windowed: false },
};

export const KINDS = Object.keys(SHAPES) as Kind[];
export const OPERATORS: Operator[] = ["LTE", "LT", "GTE", "GT", "EQ", "NEQ"];
export const WINDOWS: WindowType[] = ["NONE", "DAILY", "MONTHLY", "LIFETIME"];
export const DIRECTIONS = ["ALL", "OUT", "IN"] as const;

export const OPERATOR_SIGN: Record<Operator, string> = { EQ: "=", NEQ: "≠", GT: ">", GTE: "≥", LT: "<", LTE: "≤" };

export const BASE_LABEL: Record<Base, string> = {
  AMOUNT: "of the amount",
  FEE: "of the fee",
  NET_FEE: "of the net fee",
};

export const KIND_LABEL: Record<Kind, string> = {
  LIMIT: "Limit",
  WAIVER: "Fee waiver",
  FEE: "Fee",
  DISCOUNT: "Discount",
  TAX: "Tax",
};

export type Effect = "active" | "scheduled" | "expired";

/** Whether a policy is in effect now, starts later, or has ended. */
export function effect(value: Pick<PolicyValue, "effectiveFrom" | "effectiveTo">, now = Date.now()): Effect {
  if (Date.parse(value.effectiveFrom) > now) return "scheduled";
  if (value.effectiveTo && Date.parse(value.effectiveTo) <= now) return "expired";
  return "active";
}

/** The rule in words: "≤ 2,000,000 per day", "0.5% of the amount", "first 5 per month". */
export function describeRule(value: { kind: Kind; unit?: Unit; base?: Base; operator?: Operator; value: string; windowType: WindowType }): string {
  const amount = formatAmount(value.value);
  const per = value.windowType === "NONE" ? "" : ` per ${WINDOW_NOUN[value.windowType]}`;
  switch (value.kind) {
    case "LIMIT": {
      const what = value.unit === "COUNT" ? `${amount} transactions` : value.unit === "BALANCE" ? `${amount} balance` : amount;
      return `${value.operator ? OPERATOR_SIGN[value.operator] : ""} ${what}${per}`.trim();
    }
    case "WAIVER":
      return value.windowType === "NONE" ? "always free" : `first ${amount} free${per}`;
    default:
      return value.base ? `${amount}% ${BASE_LABEL[value.base]}` : `${amount} flat`;
  }
}

const WINDOW_NOUN: Record<WindowType, string> = { NONE: "", DAILY: "day", MONTHLY: "month", LIFETIME: "lifetime" };

const grouped = new Intl.NumberFormat("en-US", { maximumFractionDigits: 4 });

/** Groups thousands of a decimal string without going through a float. */
export function formatAmount(value: string | undefined): string {
  if (value === undefined || value === "") return "";
  const [whole, fraction] = value.split(".");
  const sign = whole.startsWith("-") ? "-" : "";
  const digits = whole.replace("-", "");
  const head = digits.length <= 15 ? grouped.format(Number(digits)) : digits.replace(/\B(?=(\d{3})+(?!\d))/g, ",");
  const tail = fraction && /[1-9]/.test(fraction) ? `.${fraction.replace(/0+$/, "")}` : "";
  return `${sign}${head}${tail}`;
}

/** datetime-local input value (local time) from an ISO string, and back. */
export function toLocalInput(iso?: string): string {
  if (!iso) return "";
  const date = new Date(iso);
  const offset = date.getTimezoneOffset() * 60_000;
  return new Date(date.getTime() - offset).toISOString().slice(0, 16);
}

export function fromLocalInput(value: string): string | undefined {
  return value ? new Date(value).toISOString() : undefined;
}

export function formatDate(iso?: string): string {
  if (!iso) return "";
  return new Date(iso).toLocaleString("en-GB", { day: "2-digit", month: "short", year: "numeric", hour: "2-digit", minute: "2-digit" });
}
