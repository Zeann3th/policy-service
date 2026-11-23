"use client";

import { useState } from "react";
import { RefreshCw } from "lucide-react";
import { api } from "@/lib/api";
import type { ReloadResponse } from "@/lib/types";
import { Field, Notice } from "@/components/ui";

// cachecore namespaces in the service (cache_service.go)
const NAMESPACES = [
  { id: "policy:config", label: "Resolved config", hint: "What reservations read; reload after editing policies by hand." },
  { id: "policies", label: "System policies", hint: "Group policy pages and items." },
  { id: "policy:custom", label: "Custom policies", hint: "Wallet exception pages and items." },
  { id: "policy:types", label: "Policy types", hint: "Type pages and items." },
  { id: "policy:groups", label: "Policy groups", hint: "Group pages and items." },
];

export default function CachePage() {
  const [selected, setSelected] = useState<string[]>(["policy:config"]);
  const [key, setKey] = useState("");
  const [result, setResult] = useState<ReloadResponse>();
  const [error, setError] = useState<unknown>();
  const [busy, setBusy] = useState(false);

  async function reload(all: boolean) {
    setBusy(true);
    setError(undefined);
    setResult(undefined);
    try {
      // the service takes a key or namespaces, never both
      setResult(await api.cache.reload(!all && key.trim() ? { key: key.trim() } : { namespaces: all ? ["all"] : selected }));
    } catch (failure) {
      setError(failure);
    } finally {
      setBusy(false);
    }
  }

  return (
    <>
      <div className="page-head">
        <div>
          <h1>Cache</h1>
          <p>Edits through this console invalidate the cache already. Reload only after changing the database directly.</p>
        </div>
      </div>

      <div className="stack" style={{ maxWidth: 760 }}>
        <section className="card">
          <div className="card-head">
            <h2>Namespaces</h2>
            <span className="sub">{selected.length} selected</span>
          </div>
          <div className="stagger" style={{ display: "grid" }}>
            {NAMESPACES.map((namespace) => (
              <label key={namespace.id} className="check" style={{ padding: "12px 20px", borderBottom: "1px solid var(--line)", alignItems: "flex-start" }}>
                <input
                  type="checkbox"
                  checked={selected.includes(namespace.id)}
                  onChange={(event) => setSelected(event.target.checked ? [...selected, namespace.id] : selected.filter((id) => id !== namespace.id))}
                  style={{ marginTop: 3 }}
                />
                <span>
                  {namespace.label} <span className="tag">{namespace.id}</span>
                  <span className="sub">{namespace.hint}</span>
                </span>
              </label>
            ))}
          </div>
          <div className="card-pad stack">
            <Field label="Single key instead" hint="Relative to the service prefix. When set, only this key is evicted and the namespaces above are left alone.">
              <input value={key} onChange={(event) => setKey(event.target.value)} placeholder="policy:config:0:PERSONAL:VND:T1" />
            </Field>
            <div style={{ display: "flex", gap: 8 }}>
              <button type="button" className="primary" disabled={busy || (!selected.length && !key.trim())} onClick={() => reload(false)}>
                <RefreshCw size={16} className={busy ? "spin" : undefined} aria-hidden /> {key.trim() ? "Evict key" : "Reload selected"}
              </button>
              <button type="button" disabled={busy} onClick={() => reload(true)}>
                Reload everything
              </button>
            </div>
          </div>
        </section>
        <Notice error={error} ok={result && (result.reloadedNamespaces?.length ? `Reloaded ${result.reloadedNamespaces.join(", ")}.` : result.evicted ? "Key evicted." : "Key was not cached; nothing to evict.")} />
      </div>
    </>
  );
}
