"use client";

import Link from "next/link";
import { useParams, useRouter } from "next/navigation";
import { useState } from "react";
import { ChevronRight, FlaskConical, Plus } from "lucide-react";
import { api } from "@/lib/api";
import type { Policy } from "@/lib/types";
import { PolicyForm } from "@/components/PolicyForm";
import { PolicyTable } from "@/components/PolicyTable";
import { DeleteButton, Legend, Notice, useResource } from "@/components/ui";

export default function GroupPage() {
  const groupId = Number(useParams<{ groupId: string }>().groupId);
  const router = useRouter();
  const group = useResource(() => api.groups.get(groupId), [groupId]);
  const policies = useResource(() => api.policies.list(groupId), [groupId]);
  const types = useResource(() => api.types.list(), []);
  const [form, setForm] = useState<{ open: boolean; editing?: Policy }>({ open: false });
  const [error, setError] = useState<unknown>();

  const name = group.data ? `${group.data.walletType} · ${group.data.tier} · ${group.data.currency}` : `Group #${groupId}`;
  const dryRunHref = group.data ? `/dry-run?walletType=${group.data.walletType}&tier=${group.data.tier}&currency=${group.data.currency}` : "/dry-run";

  return (
    <>
      <div className="page-head">
        <div>
          <div className="crumbs">
            <Link href="/groups">Policy groups</Link>
            <ChevronRight size={14} aria-hidden />
            <span>#{groupId}</span>
          </div>
          <h1>{name}</h1>
          <p>System policies for every wallet in this group. A custom policy with the same type, scope, window and unit overrides one for a single wallet.</p>
        </div>
        <div style={{ display: "flex", gap: 8 }}>
          <Link href={dryRunHref} className="button">
            <FlaskConical size={16} aria-hidden /> Dry run
          </Link>
          <button type="button" className="primary" onClick={() => setForm({ open: true })} disabled={!types.data}>
            <Plus size={16} aria-hidden /> New policy
          </button>
        </div>
      </div>

      <div className="stack">
        <Notice error={group.error ?? policies.error ?? types.error ?? error} />
        <section className="card">
          <div className="card-head">
            <h2>{policies.data ? `${policies.data.total} ${policies.data.total === 1 ? "policy" : "policies"}` : "Policies"}</h2>
            <Legend />
          </div>
          <PolicyTable
            rows={policies.data?.items}
            types={types.data?.items ?? []}
            loading={policies.loading || types.loading}
            onEdit={(row) => setForm({ open: true, editing: row })}
            onDelete={async (row) => {
              setError(undefined);
              try {
                await api.policies.remove(groupId, row.id);
                await policies.reload();
              } catch (failure) {
                setError(failure);
              }
            }}
            empty={
              <>
                <strong>No policies in this group</strong>
                <p>Without any policy, reservations for these wallets are rejected as having no applicable policy.</p>
                <button type="button" className="primary" onClick={() => setForm({ open: true })}>
                  <Plus size={16} aria-hidden /> New policy
                </button>
              </>
            }
          />
        </section>

        <section className="card card-pad" style={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 16 }}>
          <div>
            <h2>Delete this group</h2>
            <p className="sub">Removes the group and every system policy in it. Custom policies stay.</p>
          </div>
          <DeleteButton
            what="group and its policies"
            onDelete={async () => {
              try {
                await api.groups.remove(groupId);
                router.push("/groups");
              } catch (failure) {
                setError(failure);
              }
            }}
          />
        </section>
      </div>

      <PolicyForm
        open={form.open}
        editing={form.editing}
        onClose={() => setForm({ open: false })}
        types={types.data?.items ?? []}
        scope={name}
        onCreate={async (input) => {
          await api.policies.create(groupId, input);
          await policies.reload();
        }}
        onUpdate={async (id, input) => {
          await api.policies.update(groupId, id, input);
          await policies.reload();
        }}
      />
    </>
  );
}
