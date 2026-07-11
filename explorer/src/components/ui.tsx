import * as React from "react";
import * as Tooltip from "@radix-ui/react-tooltip";
import * as Tabs from "@radix-ui/react-tabs";
import { Link } from "@tanstack/react-router";
import { useUiStore } from "@/stores/ui";
import {
  absoluteUtc,
  formatNumber,
  formatPercent,
  relativeTime,
  truncateHex,
} from "@/lib/format";

export function CopyButton({ value, label = "Copy" }: { value: string; label?: string }) {
  const toast = useUiStore((s) => s.toast);
  return (
    <button
      type="button"
      className="surface-sm inline-flex h-6 items-center border border-[var(--color-line)] bg-panel px-1.5 text-[11px] text-muted hover:border-cyan hover:text-cyan"
      aria-label={label}
      onClick={async () => {
        try {
          await navigator.clipboard.writeText(value);
          toast("Copied");
        } catch {
          toast("Copy failed");
        }
      }}
    >
      Copy
    </button>
  );
}

export function HashLink({
  hash,
  kind = "tx",
  mono = true,
}: {
  hash: string;
  kind?: "tx" | "block";
  mono?: boolean;
}) {
  const h = hash.toLowerCase();
  const cls = `${mono ? "mono" : ""} truncate text-sm text-cyan hover:underline`;
  return (
    <span className="inline-flex max-w-full items-center gap-1.5">
      {kind === "block" ? (
        <Link to="/block/$id" params={{ id: h }} className={cls}>
          {truncateHex(hash)}
        </Link>
      ) : (
        <Link to="/tx/$hash" params={{ hash: h }} className={cls}>
          {truncateHex(hash)}
        </Link>
      )}
      <CopyButton value={h} />
    </span>
  );
}

export function AddressLink({ address }: { address: string }) {
  return (
    <span className="inline-flex max-w-full items-center gap-1.5">
      <Link
        to="/address/$addr"
        params={{ addr: address.toLowerCase() }}
        className="mono truncate text-sm text-cyan hover:underline"
      >
        {truncateHex(address)}
      </Link>
      <CopyButton value={address.toLowerCase()} />
    </span>
  );
}

export function StatusPill({
  status,
}: {
  status: "success" | "failed" | "pending" | "final" | "contract";
}) {
  const map = {
    success:
      "bg-[var(--ex-status-ok-bg)] text-success border-[var(--ex-status-ok-border)]",
    final:
      "bg-[var(--ex-status-final-bg)] text-cyan border-[var(--ex-status-final-border)]",
    failed:
      "bg-[var(--ex-status-fail-bg)] text-danger border-[var(--ex-status-fail-border)]",
    pending:
      "bg-[var(--ex-status-pend-bg)] text-gold border-[var(--ex-status-pend-border)]",
    contract:
      "bg-[var(--ex-status-contract-bg)] text-slate border-[var(--ex-status-contract-border)]",
  } as const;
  const text = {
    success: "Success",
    final: "Final",
    failed: "Failed",
    pending: "Pending",
    contract: "Contract Creation",
  } as const;
  return (
    <span className={`chip border ${map[status]}`}>{text[status]}</span>
  );
}

export function GasBar({ used, limit }: { used: bigint; limit: bigint }) {
  const pct = Math.min(100, formatPercent(used, limit));
  return (
    <div className="w-full max-w-md">
      <div className="mb-1 flex justify-between text-xs text-muted">
        <span className="mono">
          {formatNumber(used)} / {formatNumber(limit)}
        </span>
        <span>{pct.toFixed(2)}%</span>
      </div>
      <div className="surface-sm h-2 overflow-hidden bg-ink-soft">
        <div
          className="h-full bg-teal transition-[width] duration-200"
          style={{ width: `${pct}%`, borderRadius: "inherit" }}
        />
      </div>
    </div>
  );
}

export function TimeAgo({ tsSec }: { tsSec: number }) {
  const [, setTick] = React.useState(0);
  React.useEffect(() => {
    const id = window.setInterval(() => setTick((t) => t + 1), 15_000);
    return () => window.clearInterval(id);
  }, []);
  return (
    <span title={absoluteUtc(tsSec)} className="text-sm">
      <span className="text-frost">{relativeTime(tsSec)}</span>
      <span className="mt-0.5 block text-xs text-muted">{absoluteUtc(tsSec)}</span>
    </span>
  );
}

export function DataField({
  label,
  children,
  hint,
}: {
  label: string;
  children: React.ReactNode;
  hint?: string;
}) {
  return (
    <div className="grid gap-1 border-b border-[var(--color-line)] py-3 last:border-b-0 sm:grid-cols-[12.5rem_1fr] sm:gap-6">
      <div className="label flex items-start gap-1 pt-0.5">
        {label}
        {hint ? (
          <Tooltip.Provider delayDuration={200}>
            <Tooltip.Root>
              <Tooltip.Trigger asChild>
                <button
                  type="button"
                  className="text-muted hover:text-cyan"
                  aria-label="Help"
                >
                  ?
                </button>
              </Tooltip.Trigger>
              <Tooltip.Portal>
                <Tooltip.Content
                  className="surface-md z-50 max-w-xs border border-[var(--color-line)] bg-panel px-2 py-1.5 text-xs text-frost shadow-md"
                  sideOffset={4}
                >
                  {hint}
                </Tooltip.Content>
              </Tooltip.Portal>
            </Tooltip.Root>
          </Tooltip.Provider>
        ) : null}
      </div>
      <div className="min-w-0 break-all text-sm text-frost">{children}</div>
    </div>
  );
}

export function FieldList({ children }: { children: React.ReactNode }) {
  return <div className="panel px-4 sm:px-5">{children}</div>;
}

export function StatCard({
  label,
  value,
  sub,
}: {
  label: string;
  value: React.ReactNode;
  sub?: React.ReactNode;
}) {
  return (
    <div className="panel p-3 sm:p-4">
      <div className="mb-1 text-xs font-medium uppercase tracking-wide text-muted">{label}</div>
      <div className="text-lg font-semibold text-frost sm:text-xl">{value}</div>
      {sub ? <div className="mt-1 text-xs text-muted">{sub}</div> : null}
    </div>
  );
}

export function EmptyState({
  title,
  detail,
  action,
}: {
  title: string;
  detail?: string;
  action?: React.ReactNode;
}) {
  return (
    <div className="panel flex flex-col items-start gap-2 p-6">
      <h2 className="text-lg font-semibold text-frost">{title}</h2>
      {detail ? <p className="text-sm text-muted">{detail}</p> : null}
      {action}
    </div>
  );
}

export function ErrorBanner({ children }: { children: React.ReactNode }) {
  return (
    <div
      role="alert"
      className="surface-md mb-4 border border-[var(--ex-error-border)] bg-[var(--ex-error-bg)] px-4 py-3 text-sm text-danger"
    >
      {children}
    </div>
  );
}

export function WarningBanner({ children }: { children: React.ReactNode }) {
  return (
    <div
      role="status"
      className="surface-md mb-4 border border-[var(--ex-warn-border)] bg-[var(--ex-warn-bg)] px-4 py-3 text-sm text-gold"
    >
      {children}
    </div>
  );
}

export function PageHeader({
  title,
  badges,
  actions,
}: {
  title: React.ReactNode;
  badges?: React.ReactNode;
  actions?: React.ReactNode;
}) {
  return (
    <div className="mb-4 flex flex-col gap-2 border-b border-[var(--color-line)] pb-3 sm:flex-row sm:items-center sm:justify-between">
      <div>
        <h1 className="text-xl font-medium text-frost sm:text-2xl">{title}</h1>
        {badges ? <div className="mt-2 flex flex-wrap gap-2">{badges}</div> : null}
      </div>
      {actions}
    </div>
  );
}

export function LoadingBlock({ label = "Loading…" }: { label?: string }) {
  return (
    <div className="panel animate-pulse p-6 text-sm text-muted">{label}</div>
  );
}

export { Tabs };
