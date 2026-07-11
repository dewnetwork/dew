import { useEffect } from "react";
import { Link, Outlet } from "@tanstack/react-router";
import { config } from "@/lib/config";
import { useChainOk, useClientVersion, useHead } from "@/hooks/use-chain";
import { BrandLogo } from "./brand-logo";
import { SearchBox } from "./search-box";
import { ThemeToggle } from "./theme-toggle";
import { ErrorBanner, WarningBanner } from "./ui";
import { initThemeListeners, useUiStore } from "@/stores/ui";
import { shortError } from "@/lib/format";

export function ExplorerShell() {
  const chain = useChainOk();
  const head = useHead();
  const client = useClientVersion();
  const toasts = useUiStore((s) => s.toasts);

  useEffect(() => initThemeListeners(), []);

  return (
    <div className="flex min-h-dvh flex-col">
      {/* Status strip */}
      <div
        className="text-xs"
        style={{ background: "var(--ex-topbar)", color: "var(--ex-topbar-text)" }}
      >
        <div className="mx-auto flex max-w-6xl flex-wrap items-center justify-between gap-2 px-4 py-2 sm:px-6">
          <span className="inline-flex flex-wrap items-center gap-2">
            <span className="live-dot" aria-hidden />
            <span className="font-medium" style={{ color: "#fff" }}>
              {config.networkName}
            </span>
            <span style={{ color: "var(--ex-topbar-muted)" }}>·</span>
            <span>
              Chain{" "}
              <span className="mono font-medium" style={{ color: "#fff" }}>
                {config.expectedChainId}
              </span>
            </span>
            {head.data != null ? (
              <>
                <span style={{ color: "var(--ex-topbar-muted)" }}>·</span>
                <span>
                  Block{" "}
                  <Link
                    to="/block/$id"
                    params={{ id: String(head.data) }}
                    className="mono font-medium no-underline hover:underline"
                    style={{ color: "var(--ex-topbar-link)" }}
                  >
                    #{head.data.toLocaleString()}
                  </Link>
                </span>
              </>
            ) : null}
          </span>
          <span
            className="rounded-full px-2 py-0.5 text-[11px] font-medium"
            style={{
              color: "var(--ex-topbar-link)",
              background: "rgb(94 234 212 / 0.12)",
            }}
          >
            {config.freezeTag}
          </span>
        </div>
      </div>

      {/* Main header */}
      <header
        className="sticky top-0 z-40 border-b border-[var(--color-line)] backdrop-blur-md"
        style={{ background: "var(--ex-header-bg)" }}
      >
        <div className="mx-auto flex max-w-6xl flex-col gap-3 px-4 py-3.5 sm:flex-row sm:items-center sm:gap-5 sm:px-6">
          <Link
            to="/"
            className="group flex shrink-0 items-center gap-2.5 no-underline hover:no-underline"
          >
            <BrandLogo size={36} className="brand-mark transition-transform group-hover:scale-[1.03]" />
            <span className="leading-tight">
              <span className="block text-lg font-semibold tracking-tight text-frost">
                Dew<span className="text-[var(--ex-accent)]">Scan</span>
              </span>
              <span className="block text-[11px] font-normal text-muted">
                Block Explorer
              </span>
            </span>
          </Link>

          <div className="min-w-0 w-full flex-1">
            <SearchBox />
          </div>

          <div className="flex items-center gap-2 self-end sm:self-auto">
            <ThemeToggle />
            <a
              href="https://github.com/entj-pham/dewchain/tree/main/docs"
              className="btn-ghost hidden !py-2 text-xs no-underline hover:no-underline sm:inline-flex"
              target="_blank"
              rel="noreferrer"
            >
              Docs
            </a>
          </div>
        </div>
      </header>

      <main className="mx-auto w-full max-w-6xl flex-1 px-4 py-6 sm:px-6">
        {chain.isError ? (
          <ErrorBanner>
            RPC error: {shortError(chain.error)}. Endpoint:{" "}
            <span className="mono">{config.rpcUrl}</span>
          </ErrorBanner>
        ) : null}
        {chain.mismatch ? (
          <ErrorBanner>
            Wrong chain ID: expected <span className="mono">{chain.expected}</span>, got{" "}
            <span className="mono">{chain.actual}</span>. Refusing to treat data as{" "}
            {config.freezeTag}.
          </ErrorBanner>
        ) : null}
        {head.isError && !chain.isError ? (
          <WarningBanner>Could not fetch head: {shortError(head.error)}</WarningBanner>
        ) : null}

        {!chain.mismatch ? <Outlet /> : null}
      </main>

      <footer className="mt-auto border-t border-[var(--color-line)] bg-panel/80 py-5">
        <div className="mx-auto flex max-w-6xl flex-col items-center gap-3 px-4 text-center text-xs text-muted sm:flex-row sm:justify-between sm:text-left sm:px-6">
          <div className="flex items-center gap-2.5">
            <BrandLogo size={28} className="brand-mark opacity-90" />
            <div>
              <div className="font-medium text-frost">DewScan</div>
              <div>Read-only · no wallet keys</div>
            </div>
          </div>
          <div className="max-w-md sm:text-right">
            <div>
              {config.symbol} · Chain {config.expectedChainId}
            </div>
            <div className="mono mt-0.5 break-all opacity-80">{config.rpcUrl}</div>
            {client.data ? (
              <div className="mono mt-0.5 opacity-70">{client.data}</div>
            ) : null}
          </div>
        </div>
      </footer>

      <div className="pointer-events-none fixed bottom-4 right-4 z-50 flex flex-col gap-2">
        {toasts.map((t) => (
          <div
            key={t.id}
            className="surface-md pointer-events-auto border border-[var(--color-line)] bg-panel px-3 py-2 text-sm text-frost shadow-lg"
          >
            {t.message}
          </div>
        ))}
      </div>
    </div>
  );
}
