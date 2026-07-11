import { Link, useParams } from "@tanstack/react-router";
import { useBlock, useReceipt, useTransaction } from "@/hooks/use-chain";
import {
  absoluteUtc,
  formatDew,
  formatGwei,
  formatNumber,
  hexToBigInt,
  hexToNumber,
  relativeTime,
} from "@/lib/format";
import { config } from "@/lib/config";
import {
  AddressLink,
  CopyButton,
  DataField,
  EmptyState,
  FieldList,
  GasBar,
  LoadingBlock,
  PageHeader,
  StatusPill,
  Tabs,
} from "@/components/ui";

export function TxPage() {
  const { hash } = useParams({ from: "/tx/$hash" });
  const txQ = useTransaction(hash);
  const rcptQ = useReceipt(hash);
  const blockNum =
    txQ.data?.blockNumber != null ? hexToNumber(txQ.data.blockNumber) : undefined;
  const blockQ = useBlock(blockNum != null ? String(blockNum) : "", false);

  if (txQ.isLoading) return <LoadingBlock label="Loading transaction…" />;
  if (txQ.isError) {
    return <EmptyState title="Failed to load transaction" detail={String(txQ.error)} />;
  }
  if (!txQ.data) {
    return (
      <EmptyState
        title="Transaction not found"
        detail="Not found or not yet available on this RPC. Pending txs may appear after inclusion."
      />
    );
  }

  const tx = txQ.data;
  const rcpt = rcptQ.data;
  const pending = !tx.blockNumber || tx.blockNumber === null;
  const status: "success" | "failed" | "pending" = pending
    ? "pending"
    : rcpt?.status === "0x0"
      ? "failed"
      : "success";

  const gasLimit = hexToBigInt(tx.gas);
  const gasUsed = rcpt ? hexToBigInt(rcpt.gasUsed) : null;
  const effPrice = rcpt?.effectiveGasPrice
    ? hexToBigInt(rcpt.effectiveGasPrice)
    : tx.gasPrice
      ? hexToBigInt(tx.gasPrice)
      : tx.maxFeePerGas
        ? hexToBigInt(tx.maxFeePerGas)
        : 0n;
  const fee = gasUsed != null ? gasUsed * effPrice : null;
  const baseFee =
    blockQ.data?.baseFeePerGas != null ? hexToBigInt(blockQ.data.baseFeePerGas) : null;
  const burnt = baseFee != null && gasUsed != null ? baseFee * gasUsed : null;
  const ts = blockQ.data ? hexToNumber(blockQ.data.timestamp) : null;

  const summary =
    !tx.to
      ? "Contract creation"
      : tx.input && tx.input !== "0x"
        ? "Contract call"
        : `Transfer ${formatDew(hexToBigInt(tx.value))} ${config.symbol}`;

  return (
    <div>
      <PageHeader
        title="Transaction Details"
        badges={
          <>
            <StatusPill status={status} />
            {!pending ? <StatusPill status="final" /> : null}
          </>
        }
      />
      <p className="mb-4 text-sm text-slate">{summary}</p>

      <Tabs.Root defaultValue="overview">
        <Tabs.List className="mb-4 flex gap-1 border-b border-[var(--color-line)]">
          <Tabs.Trigger value="overview" className="tab-trigger">
            Overview
          </Tabs.Trigger>
          <Tabs.Trigger value="logs" className="tab-trigger">
            Logs ({rcpt?.logs?.length ?? 0})
          </Tabs.Trigger>
        </Tabs.List>

        <Tabs.Content value="overview">
          <FieldList>
            <DataField label="Transaction hash">
              <span className="mono break-all text-xs">{tx.hash}</span>
              <CopyButton value={tx.hash} />
            </DataField>
            <DataField label="Status">
              <StatusPill status={status} />
            </DataField>
            <DataField label="Block">
              {tx.blockNumber ? (
                <span className="inline-flex items-center gap-2">
                  <Link
                    to="/block/$id"
                    params={{ id: String(hexToNumber(tx.blockNumber)) }}
                    className="mono"
                  >
                    {hexToNumber(tx.blockNumber)}
                  </Link>
                  <StatusPill status="final" />
                </span>
              ) : (
                "—"
              )}
            </DataField>
            {ts != null ? (
              <DataField label="Timestamp">
                {relativeTime(ts)}
                <span className="mt-0.5 block text-xs text-muted">{absoluteUtc(ts)}</span>
              </DataField>
            ) : null}
            <DataField label="From">
              <AddressLink address={tx.from} />
            </DataField>
            <DataField label="To">
              {tx.to ? (
                <AddressLink address={tx.to} />
              ) : rcpt?.contractAddress ? (
                <span>
                  <StatusPill status="contract" />{" "}
                  <AddressLink address={rcpt.contractAddress} />
                </span>
              ) : (
                <StatusPill status="contract" />
              )}
            </DataField>
            <DataField label="Value">
              <span className="mono">
                {formatDew(hexToBigInt(tx.value))} {config.symbol}
                <span className="ml-2 text-xs text-muted">
                  ({hexToBigInt(tx.value).toString()} wei)
                </span>
              </span>
            </DataField>
            {fee != null ? (
              <DataField label="Transaction fee">
                <span className="mono">
                  {formatDew(fee)} {config.symbol}
                </span>
              </DataField>
            ) : null}
            <DataField label="Gas price">
              <span className="mono">{formatGwei(effPrice)} Gwei</span>
            </DataField>
            <DataField label="Gas limit & usage">
              {gasUsed != null ? (
                <GasBar used={gasUsed} limit={gasLimit} />
              ) : (
                <span className="mono">{formatNumber(gasLimit)} limit</span>
              )}
            </DataField>
            {tx.maxFeePerGas ? (
              <DataField label="EIP-1559 fees">
                <span className="mono text-xs">
                  max {formatGwei(tx.maxFeePerGas)} Gwei
                  {tx.maxPriorityFeePerGas
                    ? ` · tip ${formatGwei(tx.maxPriorityFeePerGas)} Gwei`
                    : ""}
                  {baseFee != null ? ` · base ${formatGwei(baseFee)} Gwei` : ""}
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
            <DataField label="Other attributes">
              <span className="text-xs text-muted">
                Type {tx.type ?? "0x0"} · Nonce {hexToNumber(tx.nonce)}
                {tx.transactionIndex != null
                  ? ` · Position #${hexToNumber(tx.transactionIndex)}`
                  : ""}
              </span>
            </DataField>
            <DataField label="Input data">
              <pre className="mono surface-md max-h-48 overflow-auto whitespace-pre-wrap break-all border border-[var(--color-line)] bg-ink-soft p-3 text-xs text-frost">
                {tx.input || "0x"}
              </pre>
            </DataField>
          </FieldList>
        </Tabs.Content>

        <Tabs.Content value="logs">
          {!rcpt ? (
            <EmptyState
              title={pending ? "Pending" : "No receipt yet"}
              detail="Logs appear after the transaction is included and receipt is available."
            />
          ) : rcpt.logs.length === 0 ? (
            <EmptyState title="No logs" detail="This transaction emitted no events." />
          ) : (
            <div className="flex flex-col gap-3">
              {rcpt.logs.map((log) => (
                <div key={log.logIndex} className="panel p-4">
                  <div className="mb-2 flex flex-wrap items-center gap-2 text-xs text-muted">
                    <span>Log #{hexToNumber(log.logIndex)}</span>
                    <AddressLink address={log.address} />
                  </div>
                  <div className="space-y-1">
                    {log.topics.map((t, i) => (
                      <div key={i} className="mono break-all text-xs text-frost">
                        <span className="text-muted">[{i}]</span> {t}
                      </div>
                    ))}
                    <div className="mono break-all text-xs text-muted">{log.data}</div>
                  </div>
                </div>
              ))}
            </div>
          )}
        </Tabs.Content>
      </Tabs.Root>
    </div>
  );
}
