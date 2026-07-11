import { Link, useParams } from "@tanstack/react-router";
import { useBlock } from "@/hooks/use-chain";
import {
  absoluteUtc,
  formatDew,
  formatGwei,
  formatNumber,
  hexToBigInt,
  hexToNumber,
  methodLabel,
  relativeTime,
  truncateHex,
} from "@/lib/format";
import { config } from "@/lib/config";
import type { RpcTx } from "@/lib/rpc";
import {
  AddressLink,
  CopyButton,
  DataField,
  EmptyState,
  FieldList,
  GasBar,
  HashLink,
  LoadingBlock,
  PageHeader,
  StatusPill,
  Tabs,
} from "@/components/ui";

export function BlockPage() {
  const { id } = useParams({ from: "/block/$id" });
  const q = useBlock(id, true);

  if (q.isLoading) return <LoadingBlock label="Loading block…" />;
  if (q.isError) {
    return (
      <EmptyState title="Failed to load block" detail={String(q.error)} />
    );
  }
  if (!q.data) {
    return (
      <EmptyState
        title="Block not found"
        detail={`No block for ${id}. It may be beyond head or on another chain.`}
      />
    );
  }

  const b = q.data;
  const num = b.number != null ? hexToNumber(b.number) : null;
  const gasUsed = hexToBigInt(b.gasUsed);
  const gasLimit = hexToBigInt(b.gasLimit);
  const baseFee = b.baseFeePerGas ? hexToBigInt(b.baseFeePerGas) : null;
  const burnt = baseFee != null ? baseFee * gasUsed : null;
  const txs = (b.transactions ?? []).map((t, i) =>
    typeof t === "string" ? ({ hash: t, index: i } as const) : { ...t, index: i },
  );

  return (
    <div>
      <PageHeader
        title={
          <>
            Block {num != null ? <span className="mono">#{formatNumber(num)}</span> : "details"}
          </>
        }
        badges={<StatusPill status="final" />}
        actions={
          num != null ? (
            <div className="flex gap-2">
              {num > 0 ? (
                <Link
                  to="/block/$id"
                  params={{ id: String(num - 1) }}
                  className="btn-ghost no-underline hover:no-underline"
                >
                  ← Prev
                </Link>
              ) : null}
              <Link
                to="/block/$id"
                params={{ id: String(num + 1) }}
                className="btn-ghost no-underline hover:no-underline"
              >
                Next →
              </Link>
            </div>
          ) : null
        }
      />

      <Tabs.Root defaultValue="overview">
        <Tabs.List className="mb-4 flex gap-1 border-b border-[var(--color-line)]">
          <Tabs.Trigger value="overview" className="tab-trigger">
            Overview
          </Tabs.Trigger>
          <Tabs.Trigger value="txs" className="tab-trigger">
            Transactions ({txs.length})
          </Tabs.Trigger>
        </Tabs.List>

        <Tabs.Content value="overview">
          <FieldList>
            <DataField label="Block height">
              {num != null ? formatNumber(num) : "—"}
            </DataField>
            <DataField label="Status">
              <StatusPill status="final" />
            </DataField>
            <DataField label="Timestamp">
              <span>
                {relativeTime(hexToNumber(b.timestamp))}
                <span className="mt-0.5 block text-xs text-muted">
                  {absoluteUtc(hexToNumber(b.timestamp))}
                </span>
              </span>
            </DataField>
            <DataField label="Transactions">{txs.length}</DataField>
            <DataField label="Fee recipient">
              <AddressLink address={b.miner} />
            </DataField>
            <DataField label="Gas used">
              <GasBar used={gasUsed} limit={gasLimit} />
            </DataField>
            <DataField label="Gas limit">
              <span className="mono">{formatNumber(gasLimit)}</span>
            </DataField>
            {baseFee != null ? (
              <DataField label="Base fee">
                <span className="mono">
                  {formatGwei(baseFee)} Gwei
                  <span className="ml-2 text-xs text-muted">({baseFee.toString()} wei)</span>
                </span>
              </DataField>
            ) : null}
            {burnt != null ? (
              <DataField label="Burnt fees">
                <span className="mono">
                  {formatDew(burnt)} {config.symbol}
                </span>
              </DataField>
            ) : null}
            <DataField label="Extra data">
              <span className="mono text-xs">{b.extraData}</span>
            </DataField>
            <DataField label="Hash">
              <span className="mono break-all text-xs">{b.hash}</span>
              {b.hash ? <CopyButton value={b.hash} /> : null}
            </DataField>
            <DataField label="Parent hash">
              {b.parentHash && b.parentHash !== "0x" + "0".repeat(64) ? (
                <Link
                  to="/block/$id"
                  params={{ id: b.parentHash }}
                  className="mono text-xs"
                >
                  {truncateHex(b.parentHash, 8, 8)}
                </Link>
              ) : (
                <span className="mono text-xs">{b.parentHash}</span>
              )}
            </DataField>
            <DataField label="State root">
              <span className="mono break-all text-xs">{b.stateRoot}</span>
              <CopyButton value={b.stateRoot} />
            </DataField>
            {b.receiptsRoot ? (
              <DataField label="Receipts root">
                <span className="mono break-all text-xs">{b.receiptsRoot}</span>
              </DataField>
            ) : null}
            {b.transactionsRoot ? (
              <DataField label="Transactions root">
                <span className="mono break-all text-xs">{b.transactionsRoot}</span>
              </DataField>
            ) : null}
          </FieldList>
        </Tabs.Content>

        <Tabs.Content value="txs">
          <div className="panel overflow-x-auto">
            <table className="w-full min-w-[40rem] text-left text-sm">
              <thead className="table-head">
                <tr className="border-b border-[var(--color-line)]">
                  <th className="px-4 py-2">#</th>
                  <th className="px-2 py-2">Hash</th>
                  <th className="px-2 py-2">Method</th>
                  <th className="px-2 py-2">From</th>
                  <th className="px-2 py-2">To</th>
                  <th className="px-2 py-2">Value</th>
                </tr>
              </thead>
              <tbody>
                {txs.length === 0 ? (
                  <tr>
                    <td colSpan={6} className="px-4 py-6 text-muted">
                      No transactions in this block.
                    </td>
                  </tr>
                ) : (
                  txs.map((row) => {
                    if ("from" in row && row.from) {
                      const tx = row as RpcTx & { index: number };
                      return (
                        <tr key={tx.hash} className="table-row">

                          <td className="mono px-4 py-2">{tx.index}</td>
                          <td className="px-2 py-2">
                            <HashLink hash={tx.hash} kind="tx" />
                          </td>
                          <td className="mono px-2 py-2 text-xs">
                            {methodLabel(tx.input)}
                          </td>
                          <td className="px-2 py-2">
                            <AddressLink address={tx.from} />
                          </td>
                          <td className="px-2 py-2">
                            {tx.to ? (
                              <AddressLink address={tx.to} />
                            ) : (
                              <StatusPill status="contract" />
                            )}
                          </td>
                          <td className="mono px-2 py-2">
                            {formatDew(hexToBigInt(tx.value))} {config.symbol}
                          </td>
                        </tr>
                      );
                    }
                    return (
                      <tr key={row.hash} className="table-row">
                        <td className="mono px-4 py-2">{row.index}</td>
                        <td className="px-2 py-2" colSpan={5}>
                          <HashLink hash={row.hash} kind="tx" />
                        </td>
                      </tr>
                    );
                  })
                )}
              </tbody>
            </table>
          </div>
        </Tabs.Content>
      </Tabs.Root>
    </div>
  );
}
