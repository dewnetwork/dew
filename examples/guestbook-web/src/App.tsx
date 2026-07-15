import { Fragment, useCallback, useEffect, useState, type ReactNode } from "react";
import {
  DEFAULT_CHAIN_ID,
  DEFAULT_EXPLORER,
  DEFAULT_GUESTBOOK,
  DEFAULT_RPC_URL,
  MAX_MESSAGE_BYTES,
  REACTION_KINDS,
} from "./config";
import {
  explorerAddressUrl,
  explorerTxUrl,
  fetchChainId,
  fetchEntries,
  formatTs,
  sanitizeExplorerBase,
  shortAddr,
  type GuestbookEntry,
} from "./rpc";
import {
  burstSignGuestbook,
  connectWallet,
  ensureChain,
  hasInjectedProvider,
  reactGuestbook,
  replyGuestbook,
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
  const [authorFilter, setAuthorFilter] = useState(() => {
    if (typeof window === "undefined") return "";
    try {
      return new URLSearchParams(window.location.search).get("author")?.trim() ?? "";
    } catch {
      return "";
    }
  });
  const [mineOnly, setMineOnly] = useState(false);
  const [replyOpenId, setReplyOpenId] = useState<number | null>(null);
  const [replyText, setReplyText] = useState("");
  const [acting, setActing] = useState(false);

  /** Keep ?author= in the URL for shareable filters (P3b). */
  useEffect(() => {
    if (typeof window === "undefined") return;
    const url = new URL(window.location.href);
    const t = authorFilter.trim();
    if (t) url.searchParams.set("author", t);
    else url.searchParams.delete("author");
    const next = url.pathname + url.search + url.hash;
    const cur = window.location.pathname + window.location.search + window.location.hash;
    if (next !== cur) {
      window.history.replaceState(null, "", next);
    }
  }, [authorFilter]);

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
        account,
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
  }, [rpcUrl, guestbook, chainId, account]);

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
  const explorerHome = sanitizeExplorerBase(explorer);
  const msgBytes = new TextEncoder().encode(message).length;
  const walletOk = hasInjectedProvider();

  const filterNeedle = authorFilter.trim().toLowerCase();
  const entryMatches = (e: GuestbookEntry) => {
    if (mineOnly && account) {
      if (e.author.toLowerCase() !== account.toLowerCase()) return false;
    }
    if (!filterNeedle) return true;
    return (
      e.author.toLowerCase().includes(filterNeedle) ||
      e.message.toLowerCase().includes(filterNeedle)
    );
  };
  const filteredEntries = entries.filter(entryMatches);

  // Roots newest-first; include root if it matches or any child matches.
  const roots = entries.filter((e) => e.isRoot);
  const childrenOf = (id: number) =>
    entries
      .filter((e) => !e.isRoot && e.parentId === String(id))
      .slice()
      .sort((a, b) => a.id - b.id);

  const visibleRoots = roots.filter((r) => {
    if (entryMatches(r)) return true;
    return childrenOf(r.id).some(entryMatches);
  });

  const onReact = async (entryId: number, kind: number) => {
    setSignError(null);
    setActing(true);
    try {
      await ensureAccount();
      const hash = await reactGuestbook(
        guestbook.trim(),
        entryId,
        kind,
        chainId,
        rpcUrl.trim(),
        explorer.trim(),
      );
      setLastTx(hash);
      setLastBurst(null);
      await load();
    } catch (e) {
      setSignError(e instanceof Error ? e.message : String(e));
    } finally {
      setActing(false);
    }
  };

  const onReply = async (parentId: number) => {
    setSignError(null);
    setActing(true);
    try {
      await ensureAccount();
      const hash = await replyGuestbook(
        guestbook.trim(),
        parentId,
        replyText,
        chainId,
        rpcUrl.trim(),
        explorer.trim(),
      );
      setLastTx(hash);
      setLastBurst(null);
      setReplyText("");
      setReplyOpenId(null);
      await load();
    } catch (e) {
      setSignError(e instanceof Error ? e.message : String(e));
    } finally {
      setActing(false);
    }
  };

  const renderEntryCard = (e: GuestbookEntry, isReply: boolean) => (
    <li
      key={e.id}
      className={`rounded-xl border border-line bg-panel-raised/90 p-4 shadow-lg ${
        isReply ? "ml-4 border-l-2 border-l-cyan/40 sm:ml-6" : ""
      }`}
    >
      <div className="mb-2 flex flex-wrap items-center justify-between gap-2 text-xs text-muted">
        <span className="font-mono text-cyan">
          #{e.id}
          {isReply ? " · reply" : ""}
        </span>
        <time dateTime={new Date(e.timestamp * 1000).toISOString()}>
          {formatTs(e.timestamp)}
        </time>
      </div>
      <p className="whitespace-pre-wrap text-[15px] leading-relaxed text-frost">{e.message}</p>
      <div className="mt-3 flex flex-wrap items-center gap-2 text-xs">
        <SafeExplorerLink
          className="font-mono text-mist underline-offset-2 hover:text-cyan hover:underline"
          href={explorerAddressUrl(explorer, e.author)}
          title={e.author}
        >
          {shortAddr(e.author)}
        </SafeExplorerLink>
      </div>
      <div className="mt-3 flex flex-wrap items-center gap-1.5">
        {REACTION_KINDS.map(({ kind, emoji, label }) => (
          <button
            key={kind}
            type="button"
            title={label}
            disabled={acting}
            onClick={() => void onReact(e.id, kind)}
            className={`rounded-full border px-2 py-0.5 text-xs transition disabled:opacity-50 ${
              e.myReactions[kind]
                ? "border-cyan bg-cyan/15 text-frost"
                : "border-line-strong text-muted hover:border-cyan hover:text-frost"
            }`}
          >
            {emoji} {e.reactions[kind] || 0}
          </button>
        ))}
        {!isReply ? (
          <button
            type="button"
            disabled={acting}
            onClick={() => {
              setReplyOpenId((cur) => (cur === e.id ? null : e.id));
              setReplyText("");
            }}
            className="ml-1 rounded-lg border border-line-strong px-2 py-0.5 text-xs text-frost transition hover:border-cyan hover:text-cyan disabled:opacity-50"
          >
            Reply
          </button>
        ) : null}
      </div>
      {replyOpenId === e.id ? (
        <div className="mt-3 flex flex-col gap-2 border-t border-line pt-3">
          <textarea
            className="min-h-16 w-full rounded-lg border border-line bg-ink-soft px-3 py-2 text-sm text-frost outline-none focus:border-cyan"
            placeholder="Write a reply…"
            value={replyText}
            onChange={(ev) => setReplyText(ev.target.value)}
          />
          <div className="flex gap-2">
            <button
              type="button"
              disabled={acting || !replyText.trim()}
              onClick={() => void onReply(e.id)}
              className="rounded-lg bg-teal px-3 py-1.5 text-xs font-semibold text-frost disabled:opacity-50"
            >
              {acting ? "Confirm…" : "Post reply"}
            </button>
            <button
              type="button"
              onClick={() => {
                setReplyOpenId(null);
                setReplyText("");
              }}
              className="rounded-lg border border-line-strong px-3 py-1.5 text-xs text-muted"
            >
              Cancel
            </button>
          </div>
        </div>
      ) : null}
    </li>
  );

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
          {contractHref ? (
            <a
              href={contractHref}
              target="_blank"
              rel="noreferrer"
              className="rounded-lg border border-line-strong px-4 py-2 text-sm font-medium text-frost transition hover:border-cyan hover:text-cyan"
            >
              Explorer
            </a>
          ) : (
            <span
              title="Set a valid https explorer base and guestbook address"
              className="rounded-lg border border-line-strong px-4 py-2 text-sm font-medium text-muted opacity-60"
            >
              Explorer
            </span>
          )}
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
            <SafeExplorerLink href={explorerTxUrl(explorer, lastTx)} className="font-mono underline-offset-2 hover:underline">
              {shortAddr(lastTx)}
            </SafeExplorerLink>
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
              <SafeExplorerLink
                href={explorerTxUrl(explorer, lastBurst.hashes[0])}
                className="underline-offset-2 hover:underline"
              >
                {shortAddr(lastBurst.hashes[0])}
              </SafeExplorerLink>
              {" · "}
              <SafeExplorerLink
                href={explorerTxUrl(explorer, lastBurst.hashes[1])}
                className="underline-offset-2 hover:underline"
              >
                {shortAddr(lastBurst.hashes[1])}
              </SafeExplorerLink>
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
        <div className="flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between">
          <div className="flex items-baseline justify-between gap-2 sm:block">
            <h2 className="font-display text-lg font-semibold text-frost">Messages</h2>
            <p className="text-xs text-muted">
              Newest roots first · {visibleRoots.length} thread
              {visibleRoots.length !== 1 ? "s" : ""}
              {filteredEntries.length !== entries.length
                ? ` · ${filteredEntries.length}/${entries.length} match filter`
                : ""}{" "}
              · max 200 · reactions + replies (P3c)
            </p>
          </div>
          <div className="flex flex-wrap items-center gap-2">
            <input
              type="search"
              value={authorFilter}
              onChange={(e) => setAuthorFilter(e.target.value)}
              placeholder="Filter author or text…"
              className="min-w-[12rem] flex-1 rounded-lg border border-line bg-ink-soft px-3 py-1.5 font-mono text-xs text-frost outline-none focus:border-cyan sm:max-w-xs"
              spellCheck={false}
              aria-label="Filter messages"
            />
            <button
              type="button"
              disabled={!account}
              title={account ? "Show only messages from the connected wallet" : "Connect wallet first"}
              onClick={() => setMineOnly((v) => !v)}
              className={`rounded-lg border px-3 py-1.5 text-xs font-medium transition disabled:opacity-40 ${
                mineOnly
                  ? "border-cyan bg-cyan/15 text-cyan"
                  : "border-line-strong text-frost hover:border-cyan hover:text-cyan"
              }`}
            >
              Mine
            </button>
            <button
              type="button"
              title="Copy share URL with current author filter"
              disabled={!authorFilter.trim()}
              onClick={async () => {
                const url = new URL(window.location.href);
                const t = authorFilter.trim();
                if (t) url.searchParams.set("author", t);
                else url.searchParams.delete("author");
                try {
                  await navigator.clipboard.writeText(url.toString());
                } catch {
                  /* ignore */
                }
              }}
              className="rounded-lg border border-line-strong px-3 py-1.5 text-xs font-medium text-frost transition hover:border-cyan hover:text-cyan disabled:opacity-40"
            >
              Share
            </button>
          </div>
        </div>

        {!loading && !error && entries.length === 0 && (
          <p className="rounded-xl border border-line bg-panel/50 px-4 py-8 text-center text-sm text-muted">
            No entries yet. Connect MetaMask and sign above, or use Foundry recipes.
          </p>
        )}

        {!loading && !error && entries.length > 0 && visibleRoots.length === 0 && (
          <p className="rounded-xl border border-line bg-panel/50 px-4 py-8 text-center text-sm text-muted">
            No messages match this filter.
            {(mineOnly || filterNeedle) && (
              <button
                type="button"
                className="mt-2 block w-full text-cyan underline-offset-2 hover:underline"
                onClick={() => {
                  setAuthorFilter("");
                  setMineOnly(false);
                }}
              >
                Clear filter
              </button>
            )}
          </p>
        )}

        <ul className="flex flex-col gap-3">
          {visibleRoots.map((root) => (
            <Fragment key={root.id}>
              {renderEntryCard(root, false)}
              {childrenOf(root.id)
                .filter((c) => entryMatches(c) || entryMatches(root))
                .map((child) => renderEntryCard(child, true))}
            </Fragment>
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
        <SafeExplorerLink className="text-slate hover:text-cyan" href={explorerHome}>
          explorer
        </SafeExplorerLink>
      </footer>
    </div>
  );
}

/** Renders an external link only when href is a pre-validated http(s) URL. */
function SafeExplorerLink({
  href,
  className,
  title,
  children,
}: {
  href: string | null;
  className?: string;
  title?: string;
  children: ReactNode;
}) {
  if (!href) {
    return (
      <span className={className} title={title}>
        {children}
      </span>
    );
  }
  return (
    <a className={className} href={href} target="_blank" rel="noreferrer" title={title}>
      {children}
    </a>
  );
}
