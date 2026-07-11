import type { ReactNode } from "react";
import { Link } from "@tanstack/react-router";
import { config } from "@/lib/config";
import { formatGwei, formatNumber } from "@/lib/format";
import { useNetworkStats } from "@/hooks/use-chain";
import { LoadingBlock } from "./ui";
import { TxHistoryChart } from "./tx-history-chart";
import { HISTORY_DAYS } from "@/lib/network-stats";

function formatUsd(n: number, digits = 2): string {
  return n.toLocaleString("en-US", {
    style: "currency",
    currency: "USD",
    minimumFractionDigits: digits,
    maximumFractionDigits: digits,
  });
}

function formatCompact(n: number): string {
  if (n >= 1e9) return `${(n / 1e9).toFixed(2)} B`;
  if (n >= 1e6) return `${(n / 1e6).toFixed(2)} M`;
  if (n >= 1e3) return `${(n / 1e3).toFixed(2)} K`;
  return formatNumber(Math.round(n));
}

function IconPrice() {
  return (
    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" aria-hidden>
      <path
        d="M12 2L3 7.5V16.5L12 22L21 16.5V7.5L12 2Z"
        stroke="currentColor"
        strokeWidth="1.6"
        strokeLinejoin="round"
      />
      <path d="M12 8v8M9.5 10.5h5M9.5 13.5h5" stroke="currentColor" strokeWidth="1.4" strokeLinecap="round" />
    </svg>
  );
}

function IconGlobe() {
  return (
    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" aria-hidden>
      <circle cx="12" cy="12" r="9" stroke="currentColor" strokeWidth="1.6" />
      <path
        d="M3 12h18M12 3c2.5 2.8 3.8 5.8 3.8 9s-1.3 6.2-3.8 9c-2.5-2.8-3.8-5.8-3.8-9s1.3-6.2 3.8-9z"
        stroke="currentColor"
        strokeWidth="1.4"
      />
    </svg>
  );
}

function IconBlocks() {
  return (
    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" aria-hidden>
      <rect x="3" y="3" width="8" height="8" rx="1.5" stroke="currentColor" strokeWidth="1.6" />
      <rect x="13" y="3" width="8" height="8" rx="1.5" stroke="currentColor" strokeWidth="1.6" />
      <rect x="3" y="13" width="8" height="8" rx="1.5" stroke="currentColor" strokeWidth="1.6" />
      <rect x="13" y="13" width="8" height="8" rx="1.5" stroke="currentColor" strokeWidth="1.6" />
    </svg>
  );
}

function IconClock() {
  return (
    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" aria-hidden>
      <circle cx="12" cy="12" r="9" stroke="currentColor" strokeWidth="1.6" />
      <path d="M12 7v5l3 2" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" />
    </svg>
  );
}

function IconGas() {
  return (
    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" aria-hidden>
      <path
        d="M7 20V6a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v14M7 20h10M10 9h4"
        stroke="currentColor"
        strokeWidth="1.6"
        strokeLinecap="round"
      />
      <path d="M17 10h1a2 2 0 0 1 2 2v5a2 2 0 1 1-4 0" stroke="currentColor" strokeWidth="1.6" />
    </svg>
  );
}

function IconShield() {
  return (
    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" aria-hidden>
      <path
        d="M12 3l8 3v6c0 5-3.5 8.5-8 9.5C7.5 20.5 4 17 4 12V6l8-3z"
        stroke="currentColor"
        strokeWidth="1.6"
        strokeLinejoin="round"
      />
    </svg>
  );
}

function StatCell({
  icon,
  label,
  children,
  className = "",
}: {
  icon: ReactNode;
  label: string;
  children: ReactNode;
  className?: string;
}) {
  return (
    <div className={`min-w-0 px-3 py-3.5 sm:px-4 ${className}`}>
      <div className="mb-1.5 flex items-center gap-1.5 text-[10px] font-semibold uppercase tracking-[0.06em] text-muted">
        <span className="text-cyan/80">{icon}</span>
        {label}
      </div>
      <div className="text-[13px] leading-snug text-frost sm:text-sm">{children}</div>
    </div>
  );
}

export function NetworkOverview() {
  const stats = useNetworkStats();

  if (stats.isLoading) {
    return <LoadingBlock label="Loading network overview…" />;
  }
  if (stats.isError || !stats.data) {
    return (
      <div className="panel mb-5 p-4 text-sm text-danger">
        Failed to load network stats
        {stats.error ? `: ${String(stats.error)}` : ""}.
      </div>
    );
  }

  const s = stats.data;
  const price = config.priceUsd;
  const change = config.priceChange24h;
  const mcap =
    config.marketCapUsd ??
    (price != null && config.totalSupply != null ? price * config.totalSupply : null);
  // gasPriceWei is decimal wei string from med gas resolver
  const gasWei = BigInt(s.gasPriceWei);
  const gasGwei = Number(formatGwei(gasWei, 6));
  const gasUsd =
    price != null ? (gasGwei * 21_000 * price) / 1e9 : null;
  const gasTitle =
    s.gasPriceSource === "tx_median"
      ? `Median effective gas from ${s.gasPriceSamples} tx(s) in last ${s.recentBlocks} blocks`
      : s.gasPriceSource === "base_fee_median"
        ? `Median baseFeePerGas over last ${s.recentBlocks} blocks (no recent txs)`
        : "Node eth_gasPrice suggestion (no recent fee samples)";

  const changeColor =
    change == null
      ? "text-muted"
      : change > 0
        ? "text-success"
        : change < 0
          ? "text-danger"
          : "text-muted";

  return (
    <section className="panel mb-5 overflow-hidden" aria-label="Network overview">
      {/* Layout like Etherscan: 3×2 stats + chart column; tokens match rest of explorer */}
      <div className="grid grid-cols-2 lg:grid-cols-[1fr_1fr_1fr_minmax(12rem,1.35fr)] lg:grid-rows-2">
        <StatCell
          icon={<IconPrice />}
          label={`${config.symbol} Price`}
          className="border-b border-[var(--color-line)] lg:border-r"
        >
          {price != null ? (
            <span className="font-semibold tracking-tight">
              {formatUsd(price)}
              {config.priceBtc != null ? (
                <span className="font-normal text-muted">
                  {" "}
                  @ {config.priceBtc.toFixed(6)} BTC
                </span>
              ) : null}
              {change != null ? (
                <span className={`font-medium ${changeColor}`}>
                  {" "}
                  ({change > 0 ? "+" : ""}
                  {change.toFixed(2)}%)
                </span>
              ) : null}
            </span>
          ) : (
            <span className="font-semibold text-muted">—</span>
          )}
        </StatCell>

        <StatCell
          icon={<IconBlocks />}
          label="Transactions"
          className="border-b border-[var(--color-line)] lg:border-r"
        >
          <span className="font-semibold tracking-tight">
            {formatCompact(s.txs14d)}
            <span className="font-normal text-muted">
              {" "}
              ({s.tps.toFixed(1)} TPS)
            </span>
          </span>
        </StatCell>

        <StatCell
          icon={<IconGas />}
          label="Med Gas Price"
          className="col-span-2 border-b border-[var(--color-line)] sm:col-span-1 lg:col-span-1 lg:border-r"
        >
          <span className="font-semibold tracking-tight" title={gasTitle}>
            {formatGwei(gasWei)} Gwei
            {gasUsd != null ? (
              <span className="font-normal text-muted">
                {" "}
                ({gasUsd < 0.01 ? "< $0.01" : `~${formatUsd(gasUsd)}`})
              </span>
            ) : (
              <span className="font-normal text-muted">
                {" "}
                (~{s.avgBlockTimeSec.toFixed(1)}s)
              </span>
            )}
          </span>
        </StatCell>

        <div className="order-last col-span-2 border-t border-[var(--color-line)] bg-panel-raised/50 px-3 py-3 sm:px-4 lg:order-none lg:col-span-1 lg:row-span-2 lg:border-l lg:border-t-0">
          <div className="mb-2">
            <span className="inline-block rounded-md bg-cyan/10 px-2 py-0.5 text-[10px] font-semibold uppercase tracking-[0.04em] text-cyan">
              Transaction History in {HISTORY_DAYS} days
            </span>
          </div>
          <TxHistoryChart data={s.history} variant="panel" />
        </div>

        <StatCell
          icon={<IconGlobe />}
          label="Market Cap"
          className="border-b border-[var(--color-line)] sm:border-b-0 lg:border-r"
        >
          {mcap != null ? (
            <span className="font-semibold tracking-tight">{formatUsd(mcap, 0)}</span>
          ) : (
            <span className="font-semibold text-muted">—</span>
          )}
        </StatCell>

        <StatCell
          icon={<IconClock />}
          label="Last Finalized Block"
          className="border-b border-[var(--color-line)] sm:border-b-0 lg:border-r"
        >
          <Link
            to="/block/$id"
            params={{ id: String(s.finalized) }}
            className="mono font-semibold no-underline hover:underline"
          >
            {formatNumber(s.finalized)}
          </Link>
        </StatCell>

        <StatCell
          icon={<IconShield />}
          label="Last Safe Block"
          className="col-span-2 sm:col-span-1 lg:col-span-1"
        >
          <Link
            to="/block/$id"
            params={{ id: String(s.safe) }}
            className="mono font-semibold no-underline hover:underline"
          >
            {formatNumber(s.safe)}
          </Link>
        </StatCell>
      </div>
    </section>
  );
}
