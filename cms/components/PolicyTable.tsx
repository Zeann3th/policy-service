"use client";

import { Pencil } from "lucide-react";
import { KIND_LABEL, describeRule, effect, formatDate } from "@/lib/policy";
import type { CustomPolicy, Policy, PolicyType } from "@/lib/types";
import { DeleteButton, LoadingRows, StateMark } from "./ui";

interface Props<T extends Policy | CustomPolicy> {
  rows?: T[];
  types: PolicyType[];
  loading: boolean;
  onEdit: (row: T) => void;
  onDelete: (row: T) => Promise<void>;
  empty: React.ReactNode;
}

/** Policies as ledger rows: state mark, type, scope, rule in words, effective range. */
export function PolicyTable<T extends Policy | CustomPolicy>({ rows, types, loading, onEdit, onDelete, empty }: Props<T>) {
  const byId = new Map(types.map((type) => [type.id, type]));
  if (!loading && rows?.length === 0) return <div className="empty">{empty}</div>;
  return (
    <div className="table-wrap">
      <table className="ledger">
        <thead>
          <tr>
            <th aria-label="State" />
            <th>Policy</th>
            <th>Scope</th>
            <th>Rule</th>
            <th>Effective</th>
            <th />
          </tr>
        </thead>
        <tbody className="stagger">
          {loading && !rows ? (
            <LoadingRows columns={6} />
          ) : (
            rows?.map((row) => {
              const type = byId.get(row.policyTypeId);
              const kind = type?.kind ?? "LIMIT";
              const state = effect(row);
              return (
                <tr key={row.id} className={state === "expired" ? "muted" : undefined}>
                  <td style={{ width: 28 }}>
                    <StateMark value={row} />
                  </td>
                  <td>
                    <span className="code">{type?.code ?? `type ${row.policyTypeId}`}</span>{" "}
                    <span className={`tag kind-${kind}`}>{KIND_LABEL[kind]}</span>
                    {"reason" in row && row.reason && <span className="sub">{row.reason}</span>}
                  </td>
                  <td>
                    {row.transactionType} · {row.channel}
                    <span className="sub">
                      {row.direction === "OUT" ? "sender" : row.direction === "IN" ? "receiver" : "both sides"}
                      {row.windowType !== "NONE" && ` · ${row.windowType.toLowerCase()}`}
                    </span>
                  </td>
                  <td>{describeRule({ ...row, kind })}</td>
                  <td>
                    {state === "scheduled" ? <span className="tag user">from {formatDate(row.effectiveFrom)}</span> : formatDate(row.effectiveFrom)}
                    <span className="sub">{row.effectiveTo ? `until ${formatDate(row.effectiveTo)}` : "no end"}</span>
                  </td>
                  <td className="actions">
                    <button type="button" className="ghost icon" onClick={() => onEdit(row)} aria-label={`Edit ${type?.code ?? "policy"}`}>
                      <Pencil size={15} aria-hidden />
                    </button>
                    <DeleteButton what={type?.code ?? "policy"} onDelete={() => onDelete(row)} />
                  </td>
                </tr>
              );
            })
          )}
        </tbody>
      </table>
    </div>
  );
}
