"use client";

import { useState } from "react";
import { Search } from "lucide-react";
import { api } from "@/lib/api";
import { KIND_LABEL, describeRule } from "@/lib/policy";
import type { Rule } from "@/lib/types";
import { Field, Notice } from "@/components/ui";

export default function ConfigPage() {
  const [query, setQuery] = useState({ walletId: "", walletType: "PERSONAL", currency: "VND", tier: "T1", transactionType: "" });
  const [rules, setRules] = useState<Rule[]>();
  const [error, setError] = useState<unknown>();
  const [busy, setBusy] = useState(false);
  const set = (key: keyof typeof query) => (event: React.ChangeEvent<HTMLInputElement>) => setQuery({ ...query, [key]: event.target.value });

  async function lookUp(event: React.FormEvent) {
    event.preventDefault();
    setBusy(true);
    setError(undefined);
    try {
      setRules((await api.config({ ...query, walletId: query.walletId.trim(), transactionType: query.transactionType.trim().toUpperCase() })).rules);
    } catch (failure) {
      setError(failure);
      setRules(undefined);
    } finally {
      setBusy(false);
    }
  }

  return (
    <>
      <div className="page-head">
        <div>
          <h1>Effective config</h1>
          <p>The rules a wallet runs under right now: its group’s policies in effect, with its own custom policies merged over them.</p>
        </div>
      </div>

      <form className="card card-pad" onSubmit={lookUp} style={{ marginBottom: 18 }}>
        <div className="row">
          <Field label="Wallet ID">
            <input value={query.walletId} onChange={set("walletId")} placeholder="e2e-w-0015" required />
          </Field>
          <Field label="Wallet type">
            <input value={query.walletType} onChange={set("walletType")} required />
          </Field>
          <Field label="Currency">
            <input value={query.currency} onChange={set("currency")} maxLength={3} required />
          </Field>
          <Field label="Tier">
            <input value={query.tier} onChange={set("tier")} required />
          </Field>
          <Field label="Transaction type" hint="Empty: all">
            <input value={query.transactionType} onChange={set("transactionType")} placeholder="PAYMENT" />
          </Field>
          <div className="field">
            <span aria-hidden>&nbsp;</span>
            <button type="submit" className="primary" disabled={busy}>
              <Search size={16} aria-hidden /> {busy ? "Looking up…" : "Look up"}
            </button>
          </div>
        </div>
      </form>

      <Notice error={error} />
      {rules && (
        <section className="card">
          <div className="card-head">
            <h2>
              {rules.length} {rules.length === 1 ? "rule" : "rules"} for {query.walletId}
            </h2>
            <span className="sub">
              {rules.filter((rule) => rule.source === "USER").length} from custom policies
            </span>
          </div>
          {rules.length === 0 ? (
            <div className="empty">
              <strong>No rules apply</strong>
              <p>A reservation for this wallet would be rejected as having no applicable policy.</p>
            </div>
          ) : (
            <div className="table-wrap">
              <table className="ledger">
                <thead>
                  <tr>
                    <th aria-label="State" />
                    <th>Policy</th>
                    <th>Scope</th>
                    <th>Rule</th>
                    <th>Source</th>
                  </tr>
                </thead>
                <tbody className="stagger">
                  {rules.map((rule, index) => (
                    <tr key={index}>
                      <td style={{ width: 28 }}>
                        <i className={`mark${rule.windowType === "NONE" ? "" : " tracked"}`} title={rule.windowType === "NONE" ? "In effect, checked per transaction" : "In effect, counts usage over a window"} role="img" aria-label={rule.windowType === "NONE" ? "per transaction" : "counts usage over a window"} />
                      </td>
                      <td>
                        <span className="code">{rule.policyType}</span> <span className={`tag kind-${rule.kind}`}>{KIND_LABEL[rule.kind]}</span>
                      </td>
                      <td>
                        {rule.transactionType} · {rule.channel}
                        <span className="sub">
                          {rule.direction === "OUT" ? "sender" : rule.direction === "IN" ? "receiver" : "both sides"}
                          {rule.windowType !== "NONE" && ` · ${rule.windowType.toLowerCase()}`}
                        </span>
                      </td>
                      <td>{describeRule(rule)}</td>
                      <td>{rule.source === "USER" ? <span className="tag user">Custom</span> : <span className="tag">Group</span>}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </section>
      )}
    </>
  );
}
