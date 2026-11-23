"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { AlertTriangle, CheckCircle2, X } from "lucide-react";
import { ApiFailure } from "@/lib/api";
import { effect, type Effect } from "@/lib/policy";
import type { PolicyValue } from "@/lib/types";

export function errorText(error: unknown): string {
  if (error instanceof ApiFailure) return error.status ? `${error.message} (${error.status})` : error.message;
  return String(error);
}

export function Notice({ error, ok }: { error?: unknown; ok?: string }) {
  if (!error && !ok) return null;
  return (
    <div className={`notice${ok ? " ok" : ""}`} role={ok ? "status" : "alert"}>
      {ok ? <CheckCircle2 size={16} aria-hidden /> : <AlertTriangle size={16} aria-hidden />}
      <span>{ok ?? errorText(error)}</span>
    </div>
  );
}

export function Field({ label, hint, children }: { label: string; hint?: string; children: React.ReactNode }) {
  return (
    <label className="field">
      <span>{label}</span>
      {children}
      {hint && <small>{hint}</small>}
    </label>
  );
}

/** A modal form: protected focus while a record is being written. */
export function Dialog({ title, subtitle, open, onClose, children, footer }: { title: string; subtitle?: string; open: boolean; onClose: () => void; children: React.ReactNode; footer: React.ReactNode }) {
  const ref = useRef<HTMLDialogElement>(null);
  useEffect(() => {
    const dialog = ref.current;
    if (!dialog) return;
    if (open && !dialog.open) dialog.showModal();
    if (!open && dialog.open) dialog.close();
  }, [open]);
  return (
    <dialog ref={ref} onClose={onClose} onCancel={onClose}>
      {open && (
        <>
          <div className="dialog-head">
            <div>
              <h2>{title}</h2>
              {subtitle && <p>{subtitle}</p>}
            </div>
            <button type="button" className="ghost icon" onClick={onClose} aria-label="Close">
              <X size={16} aria-hidden />
            </button>
          </div>
          <div className="dialog-body">{children}</div>
          <div className="dialog-foot">{footer}</div>
        </>
      )}
    </dialog>
  );
}

/** Two-step delete: the first click arms it and names what goes. */
export function DeleteButton({ what, onDelete }: { what: string; onDelete: () => Promise<void> }) {
  const [armed, setArmed] = useState(false);
  const [busy, setBusy] = useState(false);
  useEffect(() => {
    if (!armed) return;
    const timer = setTimeout(() => setArmed(false), 4000);
    return () => clearTimeout(timer);
  }, [armed]);
  if (!armed) {
    return (
      <button type="button" className="ghost danger" onClick={() => setArmed(true)}>
        Delete
      </button>
    );
  }
  return (
    <button
      type="button"
      className="danger solid"
      disabled={busy}
      title={`Deletes ${what}`}
      onClick={async () => {
        setBusy(true);
        try {
          await onDelete();
        } finally {
          setBusy(false);
          setArmed(false);
        }
      }}
    >
      {busy ? "Deleting…" : `Delete ${what}?`}
    </button>
  );
}

const MARK_TITLE: Record<Effect | "tracked" | "stateless", string> = {
  tracked: "In effect, counts usage over a window",
  stateless: "In effect, checked per transaction",
  scheduled: "Scheduled: not in effect yet",
  expired: "Expired: no longer in effect",
  active: "In effect",
};

/** The fixed state cell of a policy row. */
export function StateMark({ value }: { value: Pick<PolicyValue, "effectiveFrom" | "effectiveTo" | "windowType"> }) {
  const state = effect(value);
  const kind = state !== "active" ? state : value.windowType === "NONE" ? "stateless" : "tracked";
  return <i className={`mark ${kind}`} title={MARK_TITLE[kind]} aria-label={MARK_TITLE[kind]} role="img" />;
}

export function Legend() {
  return (
    <div className="legend" aria-label="Legend">
      <span>
        <i className="mark tracked" /> counts usage over a window
      </span>
      <span>
        <i className="mark" /> per transaction
      </span>
      <span>
        <i className="mark scheduled" /> scheduled
      </span>
      <span>
        <i className="mark expired" /> expired
      </span>
    </div>
  );
}

export function LoadingRows({ columns, rows = 5 }: { columns: number; rows?: number }) {
  return (
    <>
      {Array.from({ length: rows }, (_, row) => (
        <tr key={row}>
          {Array.from({ length: columns }, (_, column) => (
            <td key={column}>
              <div className="skeleton" style={{ width: `${40 + ((row * 7 + column * 13) % 50)}%` }} />
            </td>
          ))}
        </tr>
      ))}
    </>
  );
}

/** Loads data on mount and whenever deps change; reload() refetches. */
export function useResource<T>(load: () => Promise<T>, deps: React.DependencyList) {
  const [data, setData] = useState<T>();
  const [error, setError] = useState<unknown>();
  const [loading, setLoading] = useState(true);
  // eslint-disable-next-line react-hooks/exhaustive-deps
  const run = useCallback(load, deps);
  const reload = useCallback(async () => {
    setLoading(true);
    setError(undefined);
    try {
      setData(await run());
    } catch (failure) {
      setError(failure);
    } finally {
      setLoading(false);
    }
  }, [run]);
  useEffect(() => {
    void reload();
  }, [reload]);
  return { data, error, loading, reload, setData };
}
