import { useCallback, useEffect, useState } from "react";
import {
  DEFAULT_CHAIN_ID,
  DEFAULT_EXPLORER,
  DEFAULT_GUESTBOOK,
  DEFAULT_RPC_URL,
  MAX_MESSAGE_BYTES,
} from "./config";
import {
  explorerAddressUrl,
  explorerTxUrl,
  fetchChainId,
  fetchEntries,
  formatTs,
  shortAddr,
  type GuestbookEntry,
} from "./rpc";
import {
  burstSignGuestbook,
  connectWallet,
  ensureChain,
  hasInjectedProvider,
  signGuestbook,
  type BurstResult,
} from "./wallet";

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

  const [account, setAccount] = useState<string | null>(null);
  const [message, setMessage] = useState("");
  const [signing, setSigning] = useState(false);
  const [signError, setSignError] = useState<string | null>(null);
  const [lastTx, setLastTx] = useState<string | null>(null);
  const [lastBurst, setLastBurst] = useState<BurstResult | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const cid = await fetchChainId(rpcUrl.trim());
      setLiveChainId(cid);
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

  useEffect(() => {
    const eth = window.ethereum;
    if (!eth?.on) return;
    const onAccounts = (...args: unknown[]) => {
      const accs = args[0] as string[] | undefined;
      setAccount(accs?.[0] ?? null);
    };
    const onChain = () => {
      void load();
    };
    eth.on("accountsChanged", onAccounts);
    eth.on("chainChanged", onChain);
    return () => {
      eth.removeListener?.("accountsChanged", onAccounts);
      eth.removeListener?.("chainChanged", onChain);
    };
  }, [load]);

  const onConnect = async () => {
    setSignError(null);
    try {
      await ensureChain(chainId, rpcUrl.trim(), explorer.trim());
      const addr = await connectWallet();
      setAccount(addr);
    } catch (e) {
      setSignError(e instanceof Error ? e.message : String(e));
    }
  };

  const ensureAccount = async () => {
    if (account) return account;
    await ensureChain(chainId, rpcUrl.trim(), explorer.trim());
    const addr = await connectWallet();
    setAccount(addr);
    return addr;
  };

  const onSign = async () => {
    setSignError(null);
    setLastTx(null);
    setLastBurst(null);
    setSigning(true);
    try {
      await ensureAccount();
      const hash = await signGuestbook(
        guestbook.trim(),
        message,
        chainId,
        rpcUrl.trim(),
        explorer.trim(),
      );
      setLastTx(hash);
      setMessage("");
      await load();
    } catch (e) {
      setSignError(e instanceof Error ? e.message : String(e));
    } finally {
      setSigning(false);
    }
  };

  /** Two consecutive nonces — C1 may pack both into one block if confirmed promptly. */
  const onBurst = async () => {
    setSignError(null);
    setLastTx(null);
    setLastBurst(null);
    setSigning(true);
    try {
      await ensureAccount();
      const result = await burstSignGuestbook(
        guestbook.trim(),
        message,
        chainId,
        rpcUrl.trim(),
        explorer.trim(),
      );
      setLastBurst(result);
      setMessage("");
      await load();
    } catch (e) {
      setSignError(e instanceof Error ? e.message : String(e));
    } finally {
      setSigning(false);
    }
  };

  const contractHref = explorerAddressUrl(explorer, guestbook.trim());
  const msgBytes = new TextEncoder().encode(message).length;
  const walletOk = hasInjectedProvider();

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
              Read messages on-chain and sign with MetaMask (chain {chainId}). Faucet:{" "}
              <a
                className="text-cyan underline-offset-2 hover:underline"
                href="https://faucet-dew.fadosoft.com"
                target="_blank"
                rel="noreferrer"
              >
                faucet-dew.fadosoft.com
              </a>
              .
            </p>
          </div>
        </div>
        <div className="flex flex-wrap gap-2">
          <button
            type="button"
            onClick={() => void onConnect()}
            className="rounded-lg border border-line-strong px-4 py-2 text-sm font-medium text-frost transition hover:border-cyan hover:text-cyan"
          >
            {account ? shortAddr(account) : "Connect wallet"}
          </button>
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
        aria-label="Sign message"
      >
        <h2 className="mb-3 text-xs font-semibold tracking-wider text-muted uppercase">
          Sign on-chain
        </h2>
        {!walletOk && (
          <p className="mb-3 text-sm text-gold">
            No injected wallet detected. Install MetaMask, or use Foundry{" "}
            <code className="font-mono text-xs">SignGuestbook</code>.
          </p>
        )}
        <textarea
          className="min-h-24 w-full rounded-lg border border-line bg-ink-soft px-3 py-2 text-sm text-frost outline-none focus:border-cyan"
          placeholder="Write a short message (max 280 bytes)…"
          value={message}
          maxLength={400}
          onChange={(e) => setMessage(e.target.value)}
        />
        <div className="mt-2 flex flex-wrap items-center justify-between gap-2">
          <span
            className={`font-mono text-xs ${msgBytes > MAX_MESSAGE_BYTES ? "text-danger" : "text-muted"}`}
          >
            {msgBytes} / {MAX_MESSAGE_BYTES} bytes
          </span>
          <div className="flex flex-wrap gap-2">
            <button
              type="button"
              onClick={() => void onSign()}
              disabled={signing || !message.trim() || msgBytes > MAX_MESSAGE_BYTES}
              className="rounded-lg bg-teal px-4 py-2 text-sm font-semibold text-frost transition hover:bg-cyan-mid disabled:opacity-50"
            >
              {signing ? "Confirm in wallet…" : "Sign with MetaMask"}
            </button>
            <button
              type="button"
              onClick={() => void onBurst()}
              disabled={signing || !message.trim() || msgBytes > MAX_MESSAGE_BYTES - 8}
              title="Two txs, consecutive nonces — confirm both quickly to pack into one block (C1)"
              className="rounded-lg border border-line-strong px-4 py-2 text-sm font-medium text-frost transition hover:border-cyan hover:text-cyan disabled:opacity-50"
            >
              Burst ×2 (C1)
            </button>
          </div>
        </div>
        <p className="mt-2 text-xs text-muted">
          Burst posts two messages (<code className="font-mono">· A</code> /{" "}
          <code className="font-mono">· B</code>) with consecutive nonces. Confirm both MetaMask
          prompts quickly so Dew can pack them in one block (up to 64 txs).
        </p>
        {signError && (
          <p role="alert" className="mt-3 text-sm text-danger">
            {signError}
          </p>
        )}
        {lastTx && (
          <p className="mt-3 text-sm text-success">
            Posted.{" "}
            <a
              className="font-mono underline-offset-2 hover:underline"
              href={explorerTxUrl(explorer, lastTx)}
              target="_blank"
              rel="noreferrer"
            >
              {shortAddr(lastTx)}
            </a>
          </p>
        )}
        {lastBurst && (
          <div className="mt-3 space-y-1 text-sm text-success">
            <p>
              Burst posted
              {lastBurst.sameBlock
                ? ` · same block #${lastBurst.blockNumbers[0]}`
                : lastBurst.blockNumbers[0] != null && lastBurst.blockNumbers[1] != null
                  ? ` · blocks #${lastBurst.blockNumbers[0]} + #${lastBurst.blockNumbers[1]}`
                  : ""}
              .
            </p>
            <p className="font-mono text-xs">
              <a
                className="underline-offset-2 hover:underline"
                href={explorerTxUrl(explorer, lastBurst.hashes[0])}
                target="_blank"
                rel="noreferrer"
              >
                {shortAddr(lastBurst.hashes[0])}
              </a>
              {" · "}
              <a
                className="underline-offset-2 hover:underline"
                href={explorerTxUrl(explorer, lastBurst.hashes[1])}
                target="_blank"
                rel="noreferrer"
              >
                {shortAddr(lastBurst.hashes[1])}
              </a>
            </p>
          </div>
        )}
      </section>

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
            <span className="font-mono text-frost">{liveChainId ?? "—"}</span>
            {liveChainId != null && liveChainId !== chainId && (
              <span className="ml-2 text-gold">≠ expected {chainId}</span>
            )}
          </span>
          {updatedAt && <span>Updated {updatedAt.toLocaleTimeString()}</span>}
          <span className="font-mono text-muted">totalEntries = {total}</span>
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
          <h2 className="font-display text-lg font-semibold text-frost">Messages</h2>
          <p className="text-xs text-muted">Newest first · max 200</p>
        </div>

        {!loading && !error && entries.length === 0 && (
          <p className="rounded-xl border border-line bg-panel/50 px-4 py-8 text-center text-sm text-muted">
            No entries yet. Connect MetaMask and sign above, or use Foundry recipes.
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
        Chain ID {chainId} · eth_call + MetaMask ·{" "}
        <a
          className="text-slate hover:text-cyan"
          href="https://faucet-dew.fadosoft.com"
          target="_blank"
          rel="noreferrer"
        >
          faucet
        </a>
        {" · "}
        <a className="text-slate hover:text-cyan" href={explorer} target="_blank" rel="noreferrer">
          explorer
        </a>
      </footer>
    </div>
  );
}
