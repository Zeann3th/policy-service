"use client";

import { useState } from "react";
import { Pencil, Plus } from "lucide-react";
import { api } from "@/lib/api";
import { KIND_LABEL, KINDS, SHAPES, formatDate } from "@/lib/policy";
import type { Kind, PolicyType } from "@/lib/types";
import { DeleteButton, Dialog, Field, LoadingRows, Notice, useResource } from "@/components/ui";

const KIND_HINT: Record<Kind, string> = {
  LIMIT: "Blocks a transaction when its amount, count or window total breaks the rule.",
  WAIVER: "Makes the fee free while the window’s transaction count is under the value.",
  FEE: "Charges a flat amount or a percent of the amount.",
  DISCOUNT: "Takes a flat amount or a percent off the fee, or gives cashback on the amount.",
  TAX: "Charges a flat amount or a percent of the amount, fee or net fee.",
};

export default function TypesPage() {
  const types = useResource(() => api.types.list(), []);
  const [editing, setEditing] = useState<PolicyType | "new">();
  const [error, setError] = useState<unknown>();

  return (
    <>
      <div className="page-head">
        <div>
          <h1>Policy types</h1>
          <p>A type is a name and a kind. How its value is read (unit, base, operator, window) is set on each policy.</p>
        </div>
        <button type="button" className="primary" onClick={() => setEditing("new")}>
          <Plus size={16} aria-hidden /> New type
        </button>
      </div>

      <div className="stack">
        <Notice error={types.error ?? error} />
        <section className="card">
          <div className="table-wrap">
            <table className="ledger">
              <thead>
                <tr>
                  <th>Code</th>
                  <th>Kind</th>
                  <th>Description</th>
                  <th>Status</th>
                  <th>Created</th>
                  <th />
                </tr>
              </thead>
              <tbody className="stagger">
                {types.loading && !types.data ? (
                  <LoadingRows columns={6} />
                ) : (
                  types.data?.items.map((type) => (
                    <tr key={type.id} className={type.active ? undefined : "muted"}>
                      <td>
                        <span className="code">{type.code}</span>
                        <span className="sub">#{type.id}</span>
                      </td>
                      <td>
                        <span className={`tag kind-${type.kind}`}>{KIND_LABEL[type.kind]}</span>
                      </td>
                      <td style={{ maxWidth: 420 }}>{type.description ?? <span className="sub">No description</span>}</td>
                      <td>{type.active ? <span className="tag ok">Active</span> : <span className="tag">Inactive</span>}</td>
                      <td>{formatDate(type.createdAt)}</td>
                      <td className="actions">
                        <button type="button" className="ghost icon" onClick={() => setEditing(type)} aria-label={`Edit ${type.code}`}>
                          <Pencil size={15} aria-hidden />
                        </button>
                        <DeleteButton
                          what={`${type.code} and its policies`}
                          onDelete={async () => {
                            setError(undefined);
                            try {
                              await api.types.remove(type.id);
                              await types.reload();
                            } catch (failure) {
                              setError(failure);
                            }
                          }}
                        />
                      </td>
                    </tr>
                  ))
                )}
              </tbody>
            </table>
          </div>
        </section>
      </div>

      {editing && <TypeForm editing={editing === "new" ? undefined : editing} onClose={() => setEditing(undefined)} onSaved={types.reload} />}
    </>
  );
}

function TypeForm({ editing, onClose, onSaved }: { editing?: PolicyType; onClose: () => void; onSaved: () => void }) {
  const [code, setCode] = useState(editing?.code ?? "");
  const [kind, setKind] = useState<Kind>(editing?.kind ?? "FEE");
  const [description, setDescription] = useState(editing?.description ?? "");
  const [active, setActive] = useState(editing?.active ?? true);
  const [error, setError] = useState<unknown>();
  const [busy, setBusy] = useState(false);
  const shape = SHAPES[kind];

  async function submit() {
    setBusy(true);
    setError(undefined);
    try {
      if (editing) await api.types.update(editing.id, { description: description.trim() || undefined, active });
      else await api.types.create({ code: code.trim(), kind, description: description.trim() || undefined, active });
      onSaved();
      onClose();
    } catch (failure) {
      setError(failure);
    } finally {
      setBusy(false);
    }
  }

  return (
    <Dialog
      open
      onClose={onClose}
      title={editing ? `Edit ${editing.code}` : "New policy type"}
      subtitle={editing ? "Code and kind are fixed once created." : undefined}
      footer={
        <>
          <button type="button" onClick={onClose}>
            Cancel
          </button>
          <button type="button" className="primary" disabled={busy || (!editing && !code.trim())} onClick={submit}>
            {busy ? "Saving…" : editing ? "Save changes" : "Create type"}
          </button>
        </>
      }
    >
      {!editing && (
        <>
          <Field label="Code" hint="Stored upper-case, e.g. TXN_CASHBACK">
            <input value={code} onChange={(event) => setCode(event.target.value)} placeholder="TXN_CASHBACK" autoFocus />
          </Field>
          <Field label="Kind" hint={KIND_HINT[kind]}>
            <div className="segmented" role="group" aria-label="Kind">
              {KINDS.map((each) => (
                <button key={each} type="button" aria-pressed={kind === each} onClick={() => setKind(each)}>
                  {KIND_LABEL[each]}
                </button>
              ))}
            </div>
          </Field>
          <p className="sub">
            Its policies take{" "}
            {[shape.units.length && `a unit (${shape.units.join(", ")})`, shape.bases.length && `an optional base (${shape.bases.join(", ")})`, shape.operator && "an operator", shape.windowed ? "any window" : "no window"]
              .filter(Boolean)
              .join(", ")}
            .
          </p>
        </>
      )}
      <Field label="Description">
        <textarea rows={2} value={description} onChange={(event) => setDescription(event.target.value)} />
      </Field>
      {editing && (
        <label className="check">
          <input type="checkbox" checked={active} onChange={(event) => setActive(event.target.checked)} />
          Active (inactive types are ignored when policies are resolved)
        </label>
      )}
      <Notice error={error} />
    </Dialog>
  );
}
