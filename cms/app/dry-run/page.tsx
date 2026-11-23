"use client";

import { useSearchParams } from "next/navigation";
import { Suspense, useState } from "react";
import { ArrowDownUp, ArrowDownLeft, ArrowUpRight, FlaskConical, RotateCcw } from "lucide-react";
import { api } from "@/lib/api";
import { KIND_LABEL, OPERATOR_SIGN, describeRule, formatAmount, formatDate } from "@/lib/policy";
import type { AppliedPolicy, DryRunRequest, DryRunResponse, DryRunSide, Party } from "@/lib/types";
import { Field, Notice } from "@/components/ui";

const TRANSACTION_TYPES = ["TRANSFER", "PAYMENT", "CASHIN", "CASHOUT", "TOPUP"];
const CHANNELS = ["ALL", "APP", "WEB", "USSD", "API"];

type Overrides = { sender: Record<string, string>; receiver: Record<string, string> };

export default function DryRunPage() {
  return (
    <Suspense>
      <DryRun />
    </Suspense>
  );
}

function DryRun() {
  const params = useSearchParams();
  const [transactionType, setTransactionType] = useState("PAYMENT");
  const [channel, setChannel] = useState("WEB");
  const [currency, setCurrency] = useState(params.get("currency") ?? "VND");
  const [amount, setAmount] = useState("2500000");
  const [sender, setSender] = useState<Party>({ walletId: "e2e-w-0015", walletType: params.get("walletType") ?? "PERSONAL", tier: params.get("tier") ?? "T1" });
  const [receiver, setReceiver] = useState<Party>({ walletId: "e2e-w-0016", walletType: "PERSONAL", tier: "T2" });
  const [overrides, setOverrides] = useState<Overrides>({ sender: {}, receiver: {} });
  const [result, setResult] = useState<DryRunResponse>();
  const [view, setView] = useState<"statement" | "json">("statement");
  const [error, setError] = useState<unknown>();
  const [busy, setBusy] = useState(false);
  const [runs, setRuns] = useState(0);

  const validAmount = /^\d+(\.\d{1,4})?$/.test(amount.trim()) && Number(amount) > 0;
  const ready = validAmount && sender.walletId.trim() && receiver.walletId.trim() && transactionType.trim();

  async function run(withOverrides = overrides) {
    setBusy(true);
    setError(undefined);
    const request: DryRunRequest = {
      transactionType: transactionType.trim().toUpperCase(),
      channel: channel.trim().toUpperCase(),
      currency: currency.trim().toUpperCase(),
      amount: amount.trim(),
      sender: trimParty(sender),
      receiver: trimParty(receiver),
      usageOverrides: withOverrides,
    };
    try {
      setResult(await api.dryRun(request));
      setRuns((count) => count + 1);
    } catch (failure) {
      setError(failure);
      setResult(undefined);
    } finally {
      setBusy(false);
    }
  }

  const overrideCount = Object.keys(overrides.sender).length + Object.keys(overrides.receiver).length;

  return (
    <>
      <div className="page-head">
        <div>
          <h1>Dry run</h1>
          <p>Evaluates a transaction exactly as a reservation would, against live usage. Nothing is saved, no usage is added and no wallet is locked.</p>
        </div>
      </div>

      <div className="dryrun">
        <form
          className="card dryrun-form"
          onSubmit={(event) => {
            event.preventDefault();
            void run();
          }}
        >
          <div className="card-head">
            <h2>Transaction</h2>
          </div>
          <div className="card-pad stack">
            <div className="row">
              <Field label="Type">
                <input list="transaction-types" value={transactionType} onChange={(event) => setTransactionType(event.target.value)} />
                <datalist id="transaction-types">
                  {TRANSACTION_TYPES.map((each) => (
                    <option key={each} value={each} />
                  ))}
                </datalist>
              </Field>
              <Field label="Channel">
                <input list="channels" value={channel} onChange={(event) => setChannel(event.target.value)} />
                <datalist id="channels">
                  {CHANNELS.map((each) => (
                    <option key={each} value={each} />
                  ))}
                </datalist>
              </Field>
            </div>
            <div className="row" style={{ gridTemplateColumns: "1fr 96px", alignItems: "start" }}>
              <Field label="Amount" hint={validAmount ? formatAmount(amount.trim()) + " " + currency.toUpperCase() : "A positive number, up to 4 decimals"}>
                <input value={amount} onChange={(event) => setAmount(event.target.value)} inputMode="decimal" aria-invalid={!validAmount} />
              </Field>
              <Field label="Currency">
                <input value={currency} onChange={(event) => setCurrency(event.target.value)} maxLength={3} />
              </Field>
            </div>

            <div className="parties">
              <PartyCard title="Sender" direction="out" value={sender} onChange={setSender} />
              <button
                type="button"
                className="icon party-swap"
                aria-label="Swap sender and receiver"
                title="Swap sender and receiver"
                onClick={() => {
                  setSender(receiver);
                  setReceiver(sender);
                  setOverrides({ sender: {}, receiver: {} });
                }}
              >
                <ArrowDownUp size={15} aria-hidden />
              </button>
              <PartyCard title="Receiver" direction="in" value={receiver} onChange={setReceiver} />
            </div>

            <button type="submit" className="primary" disabled={busy || !ready} style={{ height: 42 }}>
              <FlaskConical size={17} className={busy ? "spin" : undefined} aria-hidden />
              {busy ? "Evaluating…" : result ? "Run again" : "Run dry run"}
            </button>
            {overrideCount > 0 && (
              <button
                type="button"
                className="ghost"
                onClick={() => {
                  const cleared = { sender: {}, receiver: {} };
                  setOverrides(cleared);
                  void run(cleared);
                }}
              >
                <RotateCcw size={14} aria-hidden /> Back to live usage ({overrideCount} {overrideCount === 1 ? "override" : "overrides"})
              </button>
            )}
          </div>
        </form>

        <div className="stack">
          <Notice error={error} />
          {result ? (
            <Statement
              key={runs}
              result={result}
              currency={currency.toUpperCase()}
              view={view}
              onView={setView}
              overrides={overrides}
              onOverride={(side, policyId, value) =>
                setOverrides((current) => {
                  const next = { ...current[side] };
                  if (value === undefined) delete next[policyId];
                  else next[policyId] = value;
                  return { ...current, [side]: next };
                })
              }
              onApply={() => run()}
            />
          ) : (
            <div className="card empty">
              <FlaskConical size={28} strokeWidth={1.5} aria-hidden color="var(--accent)" />
              <strong>See what a reservation would do</strong>
              <p>Run the transaction to get each side’s statement: every policy that applied in evaluation order, the usage it read, and the fee, discount and tax it produced.</p>
            </div>
          )}
        </div>
      </div>
    </>
  );
}

function trimParty(party: Party): Party {
  return { walletId: party.walletId.trim(), walletType: party.walletType.trim().toUpperCase(), tier: party.tier.trim().toUpperCase() };
}

function PartyCard({ title, direction, value, onChange }: { title: string; direction: "out" | "in"; value: Party; onChange: (party: Party) => void }) {
  const Icon = direction === "out" ? ArrowUpRight : ArrowDownLeft;
  return (
    <fieldset className="party" style={{ margin: 0 }}>
      <legend className="party-title" style={{ padding: 0, float: "left", width: "100%" }}>
        <span className={`avatar${direction === "out" ? " out" : ""}`} style={{ width: 22, height: 22 }}>
          <Icon size={13} aria-hidden />
        </span>
        {title}
      </legend>
      <Field label="Wallet ID">
        <input value={value.walletId} onChange={(event) => onChange({ ...value, walletId: event.target.value })} />
      </Field>
      <div className="row">
        <Field label="Wallet type">
          <input value={value.walletType} onChange={(event) => onChange({ ...value, walletType: event.target.value })} />
        </Field>
        <Field label="Tier">
          <input value={value.tier} onChange={(event) => onChange({ ...value, tier: event.target.value })} />
        </Field>
      </div>
    </fieldset>
  );
}

interface StatementProps {
  result: DryRunResponse;
  currency: string;
  view: "statement" | "json";
  onView: (view: "statement" | "json") => void;
  overrides: Overrides;
  onOverride: (side: keyof Overrides, policyId: number, value?: string) => void;
  onApply: () => void;
}

interface Breach {
  side: string;
  walletId: string;
  applied: AppliedPolicy;
}

/** The passbook statement: one ruled entry per applied policy in evaluation order, a charge per line, and the verdict stamp. */
function Statement({ result, currency, view, onView, overrides, onOverride, onApply }: StatementProps) {
  const rejected = result.status === "REJECTED";
  const breaches: Breach[] = [
    ...result.sender.appliedPolicies.filter((applied) => applied.violated).map((applied) => ({ side: "Sender", walletId: result.sender.walletId, applied })),
    ...result.receiver.appliedPolicies.filter((applied) => applied.violated).map((applied) => ({ side: "Receiver", walletId: result.receiver.walletId, applied })),
  ];
  const pending = !sameOverrides(result, overrides);

  return (
    <section className="card book">
      <div className={`verdict ${rejected ? "rejected" : "reserved"}`}>
        <div className="verdict-text">
          <h2>{rejected ? "Would be rejected" : "Would be reserved"}</h2>
          {rejected ? (
            <ul className="breaches">
              {breaches.map(({ side, walletId, applied }) => (
                <li key={`${side}-${applied.policyId}`}>
                  <b>{applied.policyType}</b>
                  {applied.windowType !== "NONE" && ` ${applied.windowType.toLowerCase()}`} on {side.toLowerCase()} {walletId}:{" "}
                  <span className="nowrap">{formatAmount(applied.usage ?? "")}</span> breaks{" "}
                  <span className="nowrap">
                    {applied.operator ? OPERATOR_SIGN[applied.operator] : ""} {formatAmount(applied.value)}
                  </span>
                </li>
              ))}
            </ul>
          ) : (
            <p>
              Nothing breaks a limit. The sender would be charged <b className="nowrap">{formatAmount(charged(result.sender))} {currency}</b>, the receiver{" "}
              <b className="nowrap">{formatAmount(charged(result.receiver))} {currency}</b>.
            </p>
          )}
        </div>
        <div className={`stamp${rejected ? "" : " ok"}`} role="status">
          {result.status}
        </div>
      </div>

      <div className="book-bar">
        <span className="sub">Evaluated {formatDate(result.evaluatedAt)} · nothing saved</span>
        <div className="segmented" role="group" aria-label="View">
          <button type="button" aria-pressed={view === "statement"} onClick={() => onView("statement")}>
            Statement
          </button>
          <button type="button" aria-pressed={view === "json"} onClick={() => onView("json")}>
            JSON
          </button>
        </div>
      </div>

      {view === "json" ? (
        <pre className="raw">{JSON.stringify(result, null, 2)}</pre>
      ) : (
        <>
          <Side title="Sender" direction="out" side={result.sender} currency={currency} overrides={overrides.sender} onOverride={(id, value) => onOverride("sender", id, value)} />
          <Side title="Receiver" direction="in" side={result.receiver} currency={currency} overrides={overrides.receiver} onOverride={(id, value) => onOverride("receiver", id, value)} />
        </>
      )}

      <div className="book-foot">
        <span>Used-before cells are editable: type what the wallet has already used to test the edge, then run again.</span>
        <button type="button" className={pending ? "primary" : undefined} disabled={!pending} onClick={onApply}>
          Run with overrides
        </button>
      </div>
    </section>
  );
}

function charged(side: DryRunSide): string {
  return addDecimals(side.fee, side.tax, `-${side.discount}`);
}

function Side({ title, direction, side, currency, overrides, onOverride }: { title: string; direction: "out" | "in"; side: DryRunSide; currency: string; overrides: Record<string, string>; onOverride: (policyId: number, value?: string) => void }) {
  const usageById = new Map(side.usage.map((usage) => [usage.policyId, usage]));
  const Icon = direction === "out" ? ArrowUpRight : ArrowDownLeft;
  return (
    <div className="book-side">
      <div className="book-title">
        <div className="who">
          <span className={`avatar${direction === "out" ? " out" : ""}`}>
            <Icon size={16} aria-hidden />
          </span>
          <div>
            <b>{side.walletId}</b>
            <span className="sub">
              {title} · {side.appliedPolicies.length} {side.appliedPolicies.length === 1 ? "policy" : "policies"} applied
            </span>
          </div>
        </div>
        {side.isViolated ? <span className="tag violated">Over a limit</span> : <span className="tag ok">Within limits</span>}
      </div>
      {side.appliedPolicies.length === 0 ? (
        <p className="empty">No policy covers this side of the transaction.</p>
      ) : (
        <table className="entries">
          <colgroup>
            <col className="c-n" />
            <col />
            <col className="c-used" />
            <col className="c-after" />
            <col className="c-rule" />
            <col className="c-charge" />
          </colgroup>
          <thead>
            <tr>
              <th className="n">#</th>
              <th>Policy</th>
              <th className="num">Used before</th>
              <th className="num">After this</th>
              <th className="num">Rule</th>
              <th className="num">Charge</th>
            </tr>
          </thead>
          <tbody>
            {side.appliedPolicies.map((applied, index) => (
              <Entry key={`${applied.policyId}-${index}`} index={index} applied={applied} usage={usageById.get(applied.policyId)} override={overrides[applied.policyId]} onOverride={(value) => onOverride(applied.policyId, value)} />
            ))}
            <tr className="total">
              <td className="n" />
              <td colSpan={4} data-label="">
                Fee {formatAmount(side.fee)} · Discount {formatAmount(side.discount)} · Tax {formatAmount(side.tax)}
                {hasCappedLines(side) && <span className="sub">Discounts are capped at the fee (cashback at the amount); a waiver zeroes the fee.</span>}
              </td>
              <td className="num nowrap" data-label="Total">
                {formatAmount(charged(side))} {currency}
              </td>
            </tr>
          </tbody>
        </table>
      )}
    </div>
  );
}

function Entry({ index, applied, usage, override, onOverride }: { index: number; applied: AppliedPolicy; usage?: DryRunSide["usage"][number]; override?: string; onOverride: (value?: string) => void }) {
  const tracked = applied.windowType !== "NONE" && usage;
  const credit = applied.kind === "DISCOUNT";
  return (
    <tr className={applied.violated ? "violated" : undefined} style={{ animationDelay: `${index * 40}ms` }}>
      <td className="n">{String(index + 1).padStart(2, "0")}</td>
      <td className="policy">
        <span className="chips">
          <i className={`mark${tracked ? " tracked" : ""}`} aria-hidden />
          <b>{applied.policyType}</b>
          <span className={`tag kind-${applied.kind}`}>{KIND_LABEL[applied.kind]}</span>
          {applied.source === "USER" && <span className="tag user">Custom</span>}
        </span>
        <span className="sub" title={`${applied.transactionType} · ${applied.channel}${applied.windowStart ? ` · ${applied.windowType.toLowerCase()} from ${applied.windowStart}` : ""}`}>
          {applied.transactionType} · {applied.channel}
          {applied.windowStart && (
            <>
              {` · ${applied.windowType.toLowerCase()} from `}
              <span className="nowrap">{applied.windowStart}</span>
            </>
          )}
        </span>
      </td>
      <td className="num" data-label="Used before">
        {tracked ? (
          <label className="used" title={override !== undefined ? `Live usage is ${formatAmount(usage.live)}` : "Live usage; edit to override"}>
            <input
              className={override !== undefined ? "overridden" : undefined}
              value={override ?? usage.used}
              inputMode="decimal"
              aria-label={`Usage before this transaction for ${applied.policyType}`}
              onChange={(event) => {
                const value = event.target.value.trim();
                onOverride(value === "" || value === usage.live ? undefined : value);
              }}
            />
            {usage.overridden && <span className="sub">live {formatAmount(usage.live)}</span>}
          </label>
        ) : (
          <span className="sub">—</span>
        )}
      </td>
      <td className="num nowrap" data-label="After this">
        {applied.usage ? formatAmount(applied.usage) : <span className="sub">—</span>}
      </td>
      <td className="num rule" data-label="Rule">
        <span className="nowrap">{applied.kind === "LIMIT" && applied.operator ? `${OPERATOR_SIGN[applied.operator]} ${formatAmount(applied.value)}` : describeRule(applied)}</span>
        {applied.violated && <span className="sub breach">violated</span>}
      </td>
      <td className="num nowrap" data-label="Charge">
        {applied.charge === undefined ? <span className="sub">—</span> : `${credit && !isZero(applied.charge) ? "−" : ""}${formatAmount(applied.charge)}`}
      </td>
    </tr>
  );
}

function hasCappedLines(side: DryRunSide): boolean {
  const lines = side.appliedPolicies.filter((applied) => applied.charge !== undefined);
  const byKind = (kind: AppliedPolicy["kind"]) => addDecimals(...lines.filter((applied) => applied.kind === kind).map((applied) => applied.charge ?? "0"));
  return toUnits(byKind("DISCOUNT")) !== toUnits(side.discount) || toUnits(byKind("FEE")) !== toUnits(side.fee);
}

function sameOverrides(result: DryRunResponse, overrides: Overrides): boolean {
  const applied = (side: DryRunSide, wanted: Record<string, string>) =>
    Object.entries(wanted).every(([id, value]) => side.usage.some((usage) => String(usage.policyId) === id && usage.overridden && usage.used === value)) &&
    side.usage.filter((usage) => usage.overridden).every((usage) => wanted[usage.policyId] === usage.used);
  return applied(result.sender, overrides.sender) && applied(result.receiver, overrides.receiver);
}

function isZero(value: string): boolean {
  return toUnits(value) === 0n;
}

const SCALE = 10_000n;

/** Exact sum of decimal strings with up to 4 places (BigInt, no float). */
function addDecimals(...values: string[]): string {
  const total = values.reduce((sum, value) => sum + toUnits(value), 0n);
  const negative = total < 0n;
  const absolute = negative ? -total : total;
  const fraction = (absolute % SCALE).toString().padStart(4, "0").replace(/0+$/, "");
  return `${negative ? "-" : ""}${absolute / SCALE}${fraction ? `.${fraction}` : ""}`;
}

function toUnits(value: string): bigint {
  const negative = value.startsWith("-");
  const [whole, fraction = ""] = value.replace("-", "").split(".");
  const units = BigInt(whole || "0") * SCALE + BigInt((fraction + "0000").slice(0, 4));
  return negative ? -units : units;
}
