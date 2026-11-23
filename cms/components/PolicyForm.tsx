"use client";

import { useMemo, useState } from "react";
import type { PolicyUpdate } from "@/lib/api";
import { BASE_LABEL, DIRECTIONS, KIND_LABEL, KINDS, OPERATORS, OPERATOR_SIGN, SHAPES, WINDOWS, describeRule, fromLocalInput, toLocalInput } from "@/lib/policy";
import type { Base, CustomPolicy, Direction, Operator, Policy, PolicyType, PolicyValue, Unit, WindowType } from "@/lib/types";
import { Dialog, Field, Notice } from "./ui";

export type PolicyCreate = Partial<PolicyValue> & { reason?: string };

interface Props {
  open: boolean;
  onClose: () => void;
  types: PolicyType[];
  /** Editing this policy; creating a new one when absent. */
  editing?: Policy | CustomPolicy;
  /** Custom policies carry a reason. */
  custom?: boolean;
  scope: string;
  onCreate: (input: PolicyCreate) => Promise<void>;
  onUpdate: (id: number, input: PolicyUpdate) => Promise<void>;
}

/** Create or edit a system or custom policy. Only the value fields the kind takes are offered. */
export function PolicyForm(props: Props) {
  // remount the form for every open, so state always starts from the record
  return props.open ? <PolicyFormBody key={props.editing?.id ?? "new"} {...props} /> : null;
}

function PolicyFormBody({ open, onClose, types, editing, custom, scope, onCreate, onUpdate }: Props) {
  const [typeId, setTypeId] = useState<number>(editing?.policyTypeId ?? types.find((type) => type.active)?.id ?? 0);
  const [transactionType, setTransactionType] = useState(editing?.transactionType ?? "ALL");
  const [channel, setChannel] = useState(editing?.channel ?? "ALL");
  const [direction, setDirection] = useState<Direction>(editing?.direction ?? "OUT");
  const [windowType, setWindowType] = useState<WindowType>(editing?.windowType ?? "NONE");
  const [unit, setUnit] = useState<Unit | "">(editing?.unit ?? "");
  const [base, setBase] = useState<Base | "">(editing?.base ?? "");
  const [operator, setOperator] = useState<Operator | "">(editing?.operator ?? "");
  const [value, setValue] = useState(editing?.value ?? "");
  const [from, setFrom] = useState(toLocalInput(editing?.effectiveFrom ?? new Date().toISOString()));
  const [to, setTo] = useState(toLocalInput(editing?.effectiveTo));
  const [reason, setReason] = useState((editing as CustomPolicy | undefined)?.reason ?? "");
  const [error, setError] = useState<unknown>();
  const [busy, setBusy] = useState(false);

  const type = types.find((candidate) => candidate.id === typeId);
  const kind = type?.kind ?? "LIMIT";
  const shape = SHAPES[kind];
  // fields the kind does not take are sent empty, whatever was typed before
  const effectiveUnit = shape.units.length ? unit || shape.units[0] : "";
  const effectiveOperator = shape.operator ? operator || "LTE" : "";
  const effectiveBase = shape.bases.includes(base as Base) ? base : "";
  const effectiveWindow = shape.windowed ? windowType : "NONE";
  const validValue = /^-?\d+(\.\d{1,4})?$/.test(value.trim());

  const preview = useMemo(
    () => (validValue ? describeRule({ kind, unit: effectiveUnit || undefined, base: effectiveBase || undefined, operator: effectiveOperator || undefined, value: value.trim(), windowType: effectiveWindow }) : ""),
    [kind, effectiveUnit, effectiveBase, effectiveOperator, value, validValue, effectiveWindow],
  );

  const grouped = KINDS.map((each) => ({ kind: each, types: types.filter((candidate) => candidate.kind === each) })).filter((group) => group.types.length);

  async function submit() {
    setBusy(true);
    setError(undefined);
    const shared = {
      base: effectiveBase || undefined,
      operator: effectiveOperator || undefined,
      value: value.trim(),
      effectiveFrom: fromLocalInput(from) ?? new Date().toISOString(),
      effectiveTo: fromLocalInput(to),
      reason: custom ? reason.trim() || undefined : undefined,
    };
    try {
      if (editing) {
        await onUpdate(editing.id, shared);
      } else {
        await onCreate({ ...shared, policyTypeId: typeId, transactionType: transactionType.trim().toUpperCase() || "ALL", channel: channel.trim().toUpperCase() || "ALL", direction, windowType: effectiveWindow, unit: effectiveUnit || undefined });
      }
      onClose();
    } catch (failure) {
      setError(failure);
    } finally {
      setBusy(false);
    }
  }

  return (
    <Dialog
      open={open}
      onClose={onClose}
      title={editing ? `Edit ${type?.code ?? "policy"}` : custom ? "New custom policy" : "New policy"}
      subtitle={scope}
      footer={
        <>
          <button type="button" onClick={onClose}>
            Cancel
          </button>
          <button type="button" className="primary" disabled={busy || !validValue || !typeId} onClick={submit}>
            {busy ? "Saving…" : editing ? "Save changes" : "Create policy"}
          </button>
        </>
      }
    >
      {editing ? (
        <div className="party">
          <div className="party-title">Identity (fixed)</div>
          <div>
            <span className="code">{type?.code}</span> <span className={`tag kind-${kind}`}>{KIND_LABEL[kind]}</span>
            <span className="sub">
              {editing.transactionType} · {editing.channel} · {editing.direction} · window {editing.windowType}
              {editing.unit && ` · unit ${editing.unit}`}
            </span>
          </div>
        </div>
      ) : (
        <>
          <Field label="Policy type" hint={type?.description}>
            <select value={typeId} onChange={(event) => setTypeId(Number(event.target.value))}>
              {grouped.map((group) => (
                <optgroup key={group.kind} label={KIND_LABEL[group.kind]}>
                  {group.types.map((candidate) => (
                    <option key={candidate.id} value={candidate.id} disabled={!candidate.active}>
                      {candidate.code}
                      {!candidate.active && " (inactive)"}
                    </option>
                  ))}
                </optgroup>
              ))}
            </select>
          </Field>
          <div className="row">
            <Field label="Transaction type" hint="ALL covers every type">
              <input value={transactionType} onChange={(event) => setTransactionType(event.target.value)} placeholder="ALL" />
            </Field>
            <Field label="Channel">
              <input value={channel} onChange={(event) => setChannel(event.target.value)} placeholder="ALL" />
            </Field>
          </div>
          <div className="row">
            <Field label="Direction" hint="OUT: sender · IN: receiver">
              <div className="segmented" role="group" aria-label="Direction">
                {DIRECTIONS.map((each) => (
                  <button key={each} type="button" aria-pressed={direction === each} onClick={() => setDirection(each)}>
                    {each}
                  </button>
                ))}
              </div>
            </Field>
            {shape.windowed && (
              <Field label="Window" hint={kind === "WAIVER" ? "Counts transactions in this window" : "NONE checks each transaction alone"}>
                <select value={windowType} onChange={(event) => setWindowType(event.target.value as WindowType)}>
                  {WINDOWS.map((each) => (
                    <option key={each}>{each}</option>
                  ))}
                </select>
              </Field>
            )}
            {shape.units.length > 1 && (
              <Field label="Measures">
                <select value={effectiveUnit} onChange={(event) => setUnit(event.target.value as Unit)}>
                  {shape.units.map((each) => (
                    <option key={each} value={each}>
                      {each === "AMOUNT" ? "Amount" : each === "COUNT" ? "Number of transactions" : "Wallet balance"}
                    </option>
                  ))}
                </select>
              </Field>
            )}
          </div>
        </>
      )}

      <div className="row">
        {shape.operator && (
          <Field label="Allowed when">
            <select value={effectiveOperator} onChange={(event) => setOperator(event.target.value as Operator)}>
              {OPERATORS.map((each) => (
                <option key={each} value={each}>
                  {OPERATOR_SIGN[each]} {each}
                </option>
              ))}
            </select>
          </Field>
        )}
        {shape.bases.length > 0 && (
          <Field label="Charged as">
            <select value={effectiveBase} onChange={(event) => setBase(event.target.value as Base | "")}>
              <option value="">Flat amount</option>
              {shape.bases.map((each) => (
                <option key={each} value={each}>
                  Percent {BASE_LABEL[each]}
                </option>
              ))}
            </select>
          </Field>
        )}
        <Field label={effectiveBase ? "Percent" : kind === "WAIVER" ? "Free transactions" : "Value"} hint="Up to 4 decimal places">
          <input value={value} onChange={(event) => setValue(event.target.value)} inputMode="decimal" placeholder="0" aria-invalid={value !== "" && !validValue} />
        </Field>
      </div>

      <div className="row">
        <Field label="Effective from">
          <input type="datetime-local" value={from} onChange={(event) => setFrom(event.target.value)} />
        </Field>
        <Field label="Effective to" hint="Empty: no end">
          <input type="datetime-local" value={to} onChange={(event) => setTo(event.target.value)} />
        </Field>
      </div>

      {custom && (
        <Field label="Reason">
          <input value={reason} onChange={(event) => setReason(event.target.value)} placeholder="Why this wallet gets an exception" />
        </Field>
      )}

      {preview && (
        <Notice ok={`Reads as: ${type?.code ?? ""} ${preview}`} />
      )}
      <Notice error={error} />
    </Dialog>
  );
}
