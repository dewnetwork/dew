import { Link } from "@tanstack/react-router";
import {
  blockNumberOf,
  blockTxCount,
  useRecentActivity,
} from "@/hooks/use-chain";
import { config } from "@/lib/config";
import { formatDew, hexToBigInt, hexToNumber } from "@/lib/format";
import {
  AddressLink,
  HashLink,
  LoadingBlock,
  TimeAgo,
} from "@/components/ui";
import { NetworkOverview } from "@/components/network-overview";
import type { RpcTx } from "@/lib/rpc";

export function HomePage() {
  const recent = useRecentActivity(12);

  return (
    <div>
      <div className="mb-5">
        <h1 className="text-xl font-semibold tracking-tight text-frost sm:text-2xl">
          The {config.networkName} Blockchain Explorer
        </h1>
        <p className="mt-1 text-sm text-muted">
          Browse blocks, transactions, and accounts · stats refresh about every minute
        </p>
      </div>

      <NetworkOverview />

      {recent.isLoading ? <LoadingBlock label="Loading recent activity…" /> : null}
      {recent.isError ? (
        <p className="text-sm text-danger">Failed to load recent blocks.</p>
      ) : null}

      {recent.data ? (
        <div className="grid gap-4 lg:grid-cols-2">
          <section className="panel overflow-hidden">
            <div className="panel-header">
              <h2 className="text-sm font-semibold text-frost">Latest Blocks</h2>
            </div>
            <div className="overflow-x-auto">
              <table className="w-full min-w-[28rem] text-left text-sm">
                <thead className="table-head">
                  <tr className="border-b border-[var(--color-line)]">
                    <th className="px-4 py-2 font-medium">Block</th>
                    <th className="px-2 py-2 font-medium">Age</th>
                    <th className="px-2 py-2 font-medium">Txn</th>
                    <th className="px-2 py-2 font-medium">Fee Recipient</th>
                  </tr>
                </thead>
                <tbody>
                  {recent.data.blocks.map((b) => {
                    const n = blockNumberOf(b);
                    return (
                      <tr key={b.hash ?? n} className="table-row">
                        <td className="px-4 py-2.5">
                          <Link
                            to="/block/$id"
                            params={{ id: String(n) }}
                            className="mono font-medium"
                          >
                            {n}
                          </Link>
                        </td>
                        <td className="px-2 py-2.5">
                          <TimeAgo tsSec={hexToNumber(b.timestamp)} />
                        </td>
                        <td className="mono px-2 py-2.5">{blockTxCount(b)}</td>
                        <td className="px-2 py-2.5">
                          <AddressLink address={b.miner} />
                        </td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
            </div>
          </section>

          <section className="panel overflow-hidden">
            <div className="panel-header">
              <h2 className="text-sm font-semibold text-frost">Latest Transactions</h2>
            </div>
            <div className="overflow-x-auto">
              <table className="w-full min-w-[28rem] text-left text-sm">
                <thead className="table-head">
                  <tr className="border-b border-[var(--color-line)]">
                    <th className="px-4 py-2 font-medium">Txn Hash</th>
                    <th className="px-2 py-2 font-medium">From / To</th>
                    <th className="px-2 py-2 font-medium">Value</th>
                  </tr>
                </thead>
                <tbody>
                  {recent.data.txs.length === 0 ? (
                    <tr>
                      <td colSpan={3} className="px-4 py-6 text-muted">
                        No transactions in the recent window.
                      </td>
                    </tr>
                  ) : (
                    recent.data.txs.map((tx) => <TxRow key={tx.hash} tx={tx} />)
                  )}
                </tbody>
              </table>
            </div>
          </section>
        </div>
      ) : null}
    </div>
  );
}

function TxRow({ tx }: { tx: RpcTx }) {
  return (
    <tr className="table-row">
      <td className="px-4 py-2.5">
        <HashLink hash={tx.hash} kind="tx" />
      </td>
      <td className="px-2 py-2.5">
        <div className="flex flex-col gap-0.5 text-xs">
          <span className="inline-flex items-center gap-1">
            <span className="text-muted">From</span>
            <AddressLink address={tx.from} />
          </span>
          <span className="inline-flex items-center gap-1">
            <span className="text-muted">To</span>
            {tx.to ? (
              <AddressLink address={tx.to} />
            ) : (
              <span className="text-teal">Contract Creation</span>
            )}
          </span>
        </div>
      </td>
      <td className="mono px-2 py-2.5">
        {formatDew(hexToBigInt(tx.value))} {config.symbol}
      </td>
    </tr>
  );
}
