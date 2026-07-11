import * as React from "react";
import { useNavigate } from "@tanstack/react-router";
import { ethGetBlockByHash, ethGetTransactionByHash } from "@/lib/rpc";
import { classifySearch } from "@/lib/format";
import { useUiStore } from "@/stores/ui";

export function SearchBox({ className = "" }: { className?: string }) {
  const [q, setQ] = React.useState("");
  const [error, setError] = React.useState<string | null>(null);
  const [busy, setBusy] = React.useState(false);
  const navigate = useNavigate();
  const pushRecent = useUiStore((s) => s.pushRecent);

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError(null);
    const { kind, value } = classifySearch(q);
    if (kind === "invalid") {
      setError("Not a valid address, tx hash, or block");
      return;
    }
    setBusy(true);
    try {
      pushRecent(value);
      if (kind === "address") {
        await navigate({ to: "/address/$addr", params: { addr: value } });
        return;
      }
      if (kind === "blockNumber") {
        await navigate({ to: "/block/$id", params: { id: value } });
        return;
      }
      const tx = await ethGetTransactionByHash(value);
      if (tx) {
        await navigate({ to: "/tx/$hash", params: { hash: value } });
        return;
      }
      const block = await ethGetBlockByHash(value, false);
      if (block) {
        await navigate({ to: "/block/$id", params: { id: value } });
        return;
      }
      await navigate({ to: "/tx/$hash", params: { hash: value } });
    } catch {
      setError("Search failed — check RPC connectivity");
    } finally {
      setBusy(false);
    }
  }

  return (
    <form onSubmit={onSubmit} className={`w-full ${className}`}>
      <div className="surface-xl flex overflow-hidden border border-[var(--color-line)] bg-ink-soft/80 shadow-sm transition-[border-color,box-shadow] focus-within:border-cyan focus-within:bg-panel focus-within:shadow-[0_0_0_3px_rgb(7_132_195_/_0.12)]">
        <span
          className="flex items-center pl-3 text-muted"
          aria-hidden
        >
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
            <circle cx="11" cy="11" r="7" />
            <path d="M20 20l-3.5-3.5" strokeLinecap="round" />
          </svg>
        </span>
        <input
          type="search"
          value={q}
          onChange={(e) => {
            setQ(e.target.value);
            setError(null);
          }}
          placeholder="Search by Address / Txn Hash / Block"
          className="mono min-w-0 flex-1 border-0 bg-transparent px-2.5 py-2.5 text-sm text-frost placeholder:text-muted/80 focus:outline-none"
          autoComplete="off"
          spellCheck={false}
          aria-label="Search"
        />
        <button
          type="submit"
          disabled={busy}
          className="btn-primary m-1 border-0 px-3.5 sm:px-4"
          style={{ borderRadius: "var(--radius-md)" }}
        >
          {busy ? "…" : "Search"}
        </button>
      </div>
      {error ? (
        <p className="mt-1.5 text-xs text-danger" role="alert">
          {error}
        </p>
      ) : null}
    </form>
  );
}
