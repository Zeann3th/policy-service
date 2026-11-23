"use client";

import { useRouter, useSearchParams } from "next/navigation";
import { Suspense, useState } from "react";
import { Plus, Search } from "lucide-react";
import { api } from "@/lib/api";
import type { CustomPolicy } from "@/lib/types";
import { PolicyForm } from "@/components/PolicyForm";
import { PolicyTable } from "@/components/PolicyTable";
import { Field, Legend, Notice, useResource } from "@/components/ui";

export default function CustomPage() {
  return (
    <Suspense>
      <Custom />
    </Suspense>
  );
}

function Custom() {
  const router = useRouter();
  const walletId = useSearchParams().get("wallet") ?? "";
  const [draft, setDraft] = useState(walletId);
  const types = useResource(() => api.types.list(), []);

  return (
    <>
      <div className="page-head">
        <div>
          <h1>Custom policies</h1>
          <p>Exceptions for one wallet. A custom policy replaces the group policy with the same type, scope, window and unit, or adds a new one.</p>
        </div>
      </div>

      <form
        className="card card-pad toolbar"
        style={{ marginBottom: 18 }}
        onSubmit={(event) => {
          event.preventDefault();
          router.push(`/custom?wallet=${encodeURIComponent(draft.trim())}`);
        }}
      >
        <div className="grow">
          <Field label="Wallet ID">
            <input value={draft} onChange={(event) => setDraft(event.target.value)} placeholder="e2e-w-0015" autoFocus />
          </Field>
        </div>
        <button type="submit" className="primary" disabled={!draft.trim()}>
          <Search size={16} aria-hidden /> Open wallet
        </button>
      </form>

      {walletId ? (
        <WalletPolicies key={walletId} walletId={walletId} types={types.data?.items} typesError={types.error} />
      ) : (
        <div className="card empty">
          <strong>Look up a wallet</strong>
          <p>Enter a wallet ID to see, add or change its exceptions. Wallets without any custom policy follow their group.</p>
        </div>
      )}
    </>
  );
}

function WalletPolicies({ walletId, types, typesError }: { walletId: string; types?: import("@/lib/types").PolicyType[]; typesError?: unknown }) {
  const policies = useResource(() => api.custom.list(walletId), [walletId]);
  const [form, setForm] = useState<{ open: boolean; editing?: CustomPolicy }>({ open: false });
  const [error, setError] = useState<unknown>();

  return (
    <div className="stack">
      <Notice error={policies.error ?? typesError ?? error} />
      <section className="card">
        <div className="card-head">
          <div>
            <h2>{walletId}</h2>
            <p>{policies.data ? `${policies.data.total} custom ${policies.data.total === 1 ? "policy" : "policies"}` : "Loading…"}</p>
          </div>
          <div style={{ display: "flex", alignItems: "center", gap: 16 }}>
            <Legend />
            <button type="button" className="primary" onClick={() => setForm({ open: true })} disabled={!types}>
              <Plus size={16} aria-hidden /> New custom policy
            </button>
          </div>
        </div>
        <PolicyTable
          rows={policies.data?.items}
          types={types ?? []}
          loading={policies.loading || !types}
          onEdit={(row) => setForm({ open: true, editing: row })}
          onDelete={async (row) => {
            setError(undefined);
            try {
              await api.custom.remove(walletId, row.id);
              await policies.reload();
            } catch (failure) {
              setError(failure);
            }
          }}
          empty={
            <>
              <strong>No exceptions for this wallet</strong>
              <p>It follows its group’s policies. Add a custom policy to raise a limit, waive a fee or give a discount.</p>
              <button type="button" className="primary" onClick={() => setForm({ open: true })}>
                <Plus size={16} aria-hidden /> New custom policy
              </button>
            </>
          }
        />
      </section>

      <PolicyForm
        custom
        open={form.open}
        editing={form.editing}
        onClose={() => setForm({ open: false })}
        types={types ?? []}
        scope={`Wallet ${walletId}`}
        onCreate={async (input) => {
          await api.custom.create(walletId, input);
          await policies.reload();
        }}
        onUpdate={async (id, input) => {
          await api.custom.update(walletId, id, input);
          await policies.reload();
        }}
      />
    </div>
  );
}
