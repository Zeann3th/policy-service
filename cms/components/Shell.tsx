"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { useEffect, useState } from "react";
import { BookOpenCheck, Database, FolderTree, ListChecks, RefreshCw, ScrollText, ShieldCheck, Tags, UserCog } from "lucide-react";
import { ACTOR, API_BASE, onRequest, type LogEntry } from "@/lib/api";
import { ActivityLog } from "./ActivityLog";

const NAV = [
  { group: null, items: [{ href: "/dry-run", label: "Dry run", icon: BookOpenCheck }] },
  {
    group: "Configure",
    items: [
      { href: "/groups", label: "Policy groups", icon: FolderTree },
      { href: "/custom", label: "Custom policies", icon: UserCog },
      { href: "/types", label: "Policy types", icon: Tags },
    ],
  },
  {
    group: "Inspect",
    items: [
      { href: "/config", label: "Effective config", icon: ListChecks },
      { href: "/cache", label: "Cache", icon: RefreshCw },
    ],
  },
];

const LOG_SIZE = 50;

export function Shell({ children }: { children: React.ReactNode }) {
  const pathname = usePathname();
  const [entries, setEntries] = useState<LogEntry[]>([]);
  const [logOpen, setLogOpen] = useState(false);

  useEffect(() => onRequest((entry) => setEntries((current) => [entry, ...current].slice(0, LOG_SIZE))), []);

  return (
    <div className="shell">
      <nav className="rail" aria-label="Main">
        <div className="rail-brand">
          <span className="logo" aria-hidden>
            <ShieldCheck size={18} strokeWidth={2} />
          </span>
          <span>
            Policy Service
            <small>admin console</small>
          </span>
        </div>
        {NAV.map((section) => (
          <div key={section.group ?? "top"}>
            {section.group && <div className="rail-group">{section.group}</div>}
            {section.items.map(({ href, label, icon: Icon }) => (
              <Link key={href} href={href} className="rail-link" aria-current={pathname.startsWith(href) ? "page" : undefined}>
                <Icon size={16} strokeWidth={1.75} aria-hidden />
                {label}
              </Link>
            ))}
          </div>
        ))}
        <div className="rail-foot">
          <button type="button" onClick={() => setLogOpen((open) => !open)} aria-expanded={logOpen}>
            <ScrollText size={15} strokeWidth={1.75} aria-hidden />
            API log
            {entries.length > 0 && <span className="count">{entries.length}</span>}
          </button>
          <span title={API_BASE}>
            <Database size={12} strokeWidth={1.75} aria-hidden style={{ verticalAlign: -1, marginRight: 6 }} />
            {new URL(API_BASE).host} · writes as “{ACTOR}”
          </span>
        </div>
      </nav>
      <main className="main">{children}</main>
      {/* on narrow screens the rail is a tab bar; the log gets its own button */}
      {!logOpen && (
        <button type="button" className="log-fab" onClick={() => setLogOpen(true)} aria-expanded={false}>
          <ScrollText size={15} strokeWidth={1.75} aria-hidden />
          API log
          {entries.length > 0 && <span className="count">{entries.length}</span>}
        </button>
      )}
      {logOpen && <ActivityLog entries={entries} onClear={() => setEntries([])} onClose={() => setLogOpen(false)} />}
    </div>
  );
}
