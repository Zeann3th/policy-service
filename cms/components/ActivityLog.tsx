"use client";

import { X } from "lucide-react";
import type { LogEntry } from "@/lib/api";

/** Every API call this session, newest first, with its request and response. */
export function ActivityLog({ entries, onClear, onClose }: { entries: LogEntry[]; onClear: () => void; onClose: () => void }) {
  return (
    <section className="log" aria-label="API log">
      <div className="log-head">
        <h2>API log</h2>
        <div style={{ display: "flex", gap: 6 }}>
          <button type="button" className="ghost" onClick={onClear} disabled={entries.length === 0}>
            Clear
          </button>
          <button type="button" className="ghost icon" onClick={onClose} aria-label="Close API log">
            <X size={16} aria-hidden />
          </button>
        </div>
      </div>
      <div className="log-list">
        {entries.length === 0 ? (
          <p className="empty">Calls to the policy service appear here, with their request and response bodies.</p>
        ) : (
          entries.map((entry) => (
            <details key={entry.id}>
              <summary>
                <span className="sub">{entry.at.toLocaleTimeString("en-GB")}</span>
                <b>{entry.method}</b>
                <span style={{ overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{entry.path}</span>
                <span className={entry.status >= 200 && entry.status < 300 ? "good" : "bad"}>{entry.status || "failed"}</span>
                <span className="sub" style={{ textAlign: "right" }}>
                  {entry.ms} ms
                </span>
              </summary>
              {entry.request !== undefined && <pre>{"request  " + JSON.stringify(entry.request, null, 2)}</pre>}
              <pre>{"response " + (entry.response === undefined ? "(empty)" : JSON.stringify(entry.response, null, 2))}</pre>
            </details>
          ))
        )}
      </div>
    </section>
  );
}
