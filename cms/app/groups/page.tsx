"use client";

import Link from "next/link";
import { useMemo, useState } from "react";
import { ArrowRight, FolderPlus } from "lucide-react";
import { api } from "@/lib/api";
import { KIND_LABEL, KINDS, effect } from "@/lib/policy";
import type { PolicyGroup, PolicyType } from "@/lib/types";
import { Dialog, Field, Notice, useResource } from "@/components/ui";

export default function GroupsPage() {
  const groups = useResource(() => api.groups.list(), []);
  const types = useResource(() => api.types.list(), []);
  const [filter, setFilter] = useState("ALL");
  const [creating, setCreating] = useState(false);

  const walletTypes = useMemo(() => ["ALL", ...new Set(groups.data?.items.map((group) => group.walletType))], [groups.data]);
  const shown = groups.data?.items.filter((group) => filter === "ALL" || group.walletType === filter);

  return (
    <>
      <div className="page-head">
        <div>
          <h1>Policy groups</h1>
          <p>A group holds the system policies for one wallet type, currency and tier. Open one to manage its limits, fees and tax.</p>
        </div>
        <button type="button" className="primary" onClick={() => setCreating(true)}>
          <FolderPlus size={16} aria-hidden /> New group
        </button>
      </div>

      {walletTypes.length > 2 && (
        <div className="toolbar">
          <div className="segmented" role="group" aria-label="Wallet type">
            {walletTypes.map((each) => (
              <button key={each} type="button" aria-pressed={filter === each} onClick={() => setFilter(each)}>
                {each === "ALL" ? "All wallet types" : each}
              </button>
            ))}
          </div>
        </div>
      )}

      <Notice error={groups.error} />
      {groups.loading && !groups.data ? (
        <div className="tiles">
          {Array.from({ length: 6 }, (_, index) => (
            <div key={index} className="tile">
              <div className="skeleton" style={{ width: "60%", height: 20 }} />
              <div className="skeleton" style={{ width: "40%" }} />
            </div>
          ))}
        </div>
      ) : shown?.length === 0 ? (
        <div className="card empty">
          <strong>No policy groups yet</strong>
          <p>Create one per wallet type, currency and tier, then add its limits and fees.</p>
          <button type="button" className="primary" onClick={() => setCreating(true)}>
            <FolderPlus size={16} aria-hidden /> New group
          </button>
        </div>
      ) : (
        <div className="tiles stagger">
          {shown?.map((group) => (
            <GroupTile key={group.id} group={group} types={types.data?.items} />
          ))}
        </div>
      )}

      <CreateGroup open={creating} onClose={() => setCreating(false)} onCreated={groups.reload} />
    </>
  );
}

function CreateGroup({ open, onClose, onCreated }: { open: boolean; onClose: () => void; onCreated: () => void }) {
  const [walletType, setWalletType] = useState("PERSONAL");
  const [currency, setCurrency] = useState("VND");
  const [tier, setTier] = useState("");
  const [error, setError] = useState<unknown>();
  const [busy, setBusy] = useState(false);

  async function submit() {
    setBusy(true);
    setError(undefined);
    try {
      await api.groups.create({ walletType: walletType.trim().toUpperCase(), currency: currency.trim().toUpperCase(), tier: tier.trim().toUpperCase() });
      onCreated();
      onClose();
      setTier("");
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
      title="New policy group"
      subtitle="Wallets of this type, currency and tier share its system policies."
      footer={
        <>
          <button type="button" onClick={onClose}>
            Cancel
          </button>
          <button type="button" className="primary" disabled={busy || !walletType.trim() || !tier.trim() || currency.trim().length !== 3} onClick={submit}>
            {busy ? "Creating…" : "Create group"}
          </button>
        </>
      }
    >
      <div className="row">
        <Field label="Wallet type">
          <input value={walletType} onChange={(event) => setWalletType(event.target.value)} placeholder="PERSONAL" />
        </Field>
        <Field label="Currency" hint="ISO 4217">
          <input value={currency} onChange={(event) => setCurrency(event.target.value)} maxLength={3} placeholder="VND" />
        </Field>
        <Field label="Tier">
          <input value={tier} onChange={(event) => setTier(event.target.value)} placeholder="T1" />
        </Field>
      </div>
      <Notice error={error} />
    </Dialog>
  );
}

/** A group at a glance: how many policies of each kind, and what is not in effect. */
function GroupTile({ group, types }: { group: PolicyGroup; types?: PolicyType[] }) {
  const policies = useResource(() => api.policies.list(group.id), [group.id]);
  const kindOf = new Map(types?.map((type) => [type.id, type.kind]));
  const items = policies.data?.items ?? [];
  const counts = KINDS.map((kind) => ({ kind, count: items.filter((item) => kindOf.get(item.policyTypeId) === kind).length })).filter((entry) => entry.count);
  const scheduled = items.filter((item) => effect(item) === "scheduled").length;
  const expired = items.filter((item) => effect(item) === "expired").length;
  return (
    <Link href={`/groups/${group.id}`} className="tile">
      <div className="tile-top">
        <div>
          <div className="big">
            {group.walletType} · {group.tier}
          </div>
          <span className="sub">
            {policies.data ? `${policies.data.total} ${policies.data.total === 1 ? "policy" : "policies"}` : "Loading policies…"}
          </span>
        </div>
        <span className="tag">{group.currency}</span>
      </div>
      <div className="tile-kinds">
        {!types || policies.loading ? (
          <div className="skeleton" style={{ width: "70%" }} />
        ) : counts.length ? (
          counts.map(({ kind, count }) => (
            <span key={kind} className={`tag kind-${kind}`}>
              {count} {KIND_LABEL[kind].toLowerCase()}
            </span>
          ))
        ) : (
          <span className="sub">No policies yet</span>
        )}
      </div>
      <div className="tile-foot">
        <span className="legend">
          {scheduled > 0 && (
            <span>
              <i className="mark scheduled" /> {scheduled} scheduled
            </span>
          )}
          {expired > 0 && (
            <span>
              <i className="mark expired" /> {expired} expired
            </span>
          )}
          {!scheduled && !expired && <span>All in effect</span>}
        </span>
        <ArrowRight size={16} aria-hidden />
      </div>
    </Link>
  );
}
