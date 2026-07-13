import { useCallback, useEffect, useState } from "react";
import {
  DEFAULT_CHAIN_ID,
  DEFAULT_EXPLORER,
  DEFAULT_GUESTBOOK,
  DEFAULT_RPC_URL,
} from "./config";
import {
  explorerAddressUrl,
  fetchChainId,
  fetchEntries,
  formatTs,
  shortAddr,
  type GuestbookEntry,
} from "./rpc";

export default function App() {
  const [rpcUrl, setRpcUrl] = useState(DEFAULT_RPC_URL);
  const [guestbook, setGuestbook] = useState(DEFAULT_GUESTBOOK);
  const [explorer, setExplorer] = useState(DEFAULT_EXPLORER);
  const [chainId, setChainId] = useState(DEFAULT_CHAIN_ID);

  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [total, setTotal] = useState(0);
  const [entries, setEntries] = useState<GuestbookEntry[]>([]);
  const [liveChainId, setLiveChainId] = useState<number | null>(null);
  const [updatedAt, setUpdatedAt] = useState<Date | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const cid = await fetchChainId(rpcUrl.trim());
      setLiveChainId(cid);
      if (cid !== chainId) {
        // Still attempt reads; warn via error banner soft note
      }
      const { total: t, entries: list } = await fetchEntries(
        rpcUrl.trim(),
        guestbook.trim(),
        chainId,
      );
      setTotal(t);
      setEntries(list);
      setUpdatedAt(new Date());
    } catch (e) {
      setEntries([]);
      setTotal(0);
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setLoading(false);
    }
  }, [rpcUrl, guestbook, chainId]);

  useEffect(() => {
    void load();
  }, [load]);

  const contractHref = explorerAddressUrl(explorer, guestbook.trim());

  return (
    <div className="mx-auto flex min-h-dvh max-w-3xl flex-col gap-8 px-4 py-10 sm:px-6">
      <header className="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
        <div className="flex items-start gap-3">
          <img src="/logo.svg" alt="" className="mt-1 h-10 w-10" />
          <div>
            <p className="font-display text-xs font-semibold tracking-[0.2em] text-cyan uppercase">
              Dew · public-testnet-v1
            </p>
            <h1 className="font-display text-3xl font-bold tracking-tight text-frost sm:text-4xl">
              Guestbook
            </h1>
            <p className="mt-1 max-w-md text-sm text-muted">
              Read-only on-chain messages via JSON-RPC. Deploy or sign with Foundry —{" "}
              <a
                className="text-cyan underline-offset-2 hover:underline"
                href="https://github.com/dewnetwork/dew/blob/main/docs/ops/recipes.md"
                target="_blank"
                rel="noreferrer"
              >
                recipes
              </a>
              .
            </p>
          </div>
        </div>
        <div className="flex flex-wrap gap-2">
          <button
            type="button"
            onClick={() => void load()}
            disabled={loading}
            className="rounded-lg bg-cyan px-4 py-2 text-sm font-semibold text-ink transition hover:bg-mist disabled:opacity-50"
          >
            {loading ? "Loading…" : "Refresh"}
          </button>
          <a
            href={contractHref}
            target="_blank"
            rel="noreferrer"
            className="rounded-lg border border-line-strong px-4 py-2 text-sm font-medium text-frost transition hover:border-cyan hover:text-cyan"
          >
            Explorer
          </a>
        </div>
      </header>

      <section
        className="rounded-2xl border border-line bg-panel/80 p-4 shadow-xl backdrop-blur-sm sm:p-5"
        aria-label="Connection"
      >
        <h2 className="mb-3 text-xs font-semibold tracking-wider text-muted uppercase">
          Connection
        </h2>
        <div className="grid gap-3 sm:grid-cols-2">
          <label className="flex flex-col gap-1 text-xs text-muted">
            RPC URL
            <input
              className="rounded-lg border border-line bg-ink-soft px-3 py-2 font-mono text-sm text-frost outline-none focus:border-cyan"
              value={rpcUrl}
              onChange={(e) => setRpcUrl(e.target.value)}
              spellCheck={false}
            />
          </label>
          <label className="flex flex-col gap-1 text-xs text-muted">
            Guestbook address
            <input
              className="rounded-lg border border-line bg-ink-soft px-3 py-2 font-mono text-sm text-frost outline-none focus:border-cyan"
              value={guestbook}
              onChange={(e) => setGuestbook(e.target.value)}
              spellCheck={false}
            />
          </label>
          <label className="flex flex-col gap-1 text-xs text-muted">
            Expected chain ID
            <input
              type="number"
              className="rounded-lg border border-line bg-ink-soft px-3 py-2 font-mono text-sm text-frost outline-none focus:border-cyan"
              value={chainId}
              onChange={(e) => setChainId(Number(e.target.value) || 2205)}
            />
          </label>
          <label className="flex flex-col gap-1 text-xs text-muted">
            Explorer base
            <input
              className="rounded-lg border border-line bg-ink-soft px-3 py-2 font-mono text-sm text-frost outline-none focus:border-cyan"
              value={explorer}
              onChange={(e) => setExplorer(e.target.value)}
              spellCheck={false}
            />
          </label>
        </div>
        <div className="mt-3 flex flex-wrap items-center gap-3 text-xs text-slate">
          <span>
            Live chain:{" "}
            <span className="font-mono text-frost">
              {liveChainId ?? "—"}
            </span>
            {liveChainId != null && liveChainId !== chainId && (
              <span className="ml-2 text-gold">≠ expected {chainId}</span>
            )}
          </span>
          {updatedAt && (
            <span>
              Updated {updatedAt.toLocaleTimeString()}
            </span>
          )}
          <span className="font-mono text-muted">
            totalEntries = {total}
          </span>
        </div>
      </section>

      {error && (
        <div
          role="alert"
          className="rounded-xl border border-danger/40 bg-danger/10 px-4 py-3 text-sm text-danger"
        >
          {error}
        </div>
      )}

      <section aria-label="Entries" className="flex flex-col gap-3">
        <div className="flex items-baseline justify-between">
          <h2 className="font-display text-lg font-semibold text-frost">
            Messages
          </h2>
          <p className="text-xs text-muted">Newest first · max 200</p>
        </div>

        {!loading && !error && entries.length === 0 && (
          <p className="rounded-xl border border-line bg-panel/50 px-4 py-8 text-center text-sm text-muted">
            No entries yet. Deploy and{" "}
            <code className="font-mono text-cyan">sign()</code> via Foundry
            recipes.
          </p>
        )}

        <ul className="flex flex-col gap-3">
          {entries.map((e) => (
            <li
              key={e.id}
              className="rounded-xl border border-line bg-panel-raised/90 p-4 shadow-lg"
            >
              <div className="mb-2 flex flex-wrap items-center justify-between gap-2 text-xs text-muted">
                <span className="font-mono text-cyan">#{e.id}</span>
                <time dateTime={new Date(e.timestamp * 1000).toISOString()}>
                  {formatTs(e.timestamp)}
                </time>
              </div>
              <p className="whitespace-pre-wrap text-[15px] leading-relaxed text-frost">
                {e.message}
              </p>
              <div className="mt-3 flex flex-wrap items-center gap-2 text-xs">
                <a
                  className="font-mono text-mist underline-offset-2 hover:text-cyan hover:underline"
                  href={explorerAddressUrl(explorer, e.author)}
                  target="_blank"
                  rel="noreferrer"
                  title={e.author}
                >
                  {shortAddr(e.author)}
                </a>
              </div>
            </li>
          ))}
        </ul>
      </section>

      <footer className="mt-auto border-t border-line pt-6 text-center text-xs text-muted">
        Chain ID {chainId} · read-only eth_call ·{" "}
        <a
          className="text-slate hover:text-cyan"
          href="https://faucet-dew.fadosoft.com"
          target="_blank"
          rel="noreferrer"
        >
          faucet
        </a>
        {" · "}
        <a
          className="text-slate hover:text-cyan"
          href={explorer}
          target="_blank"
          rel="noreferrer"
        >
          explorer
        </a>
      </footer>
    </div>
  );
}
