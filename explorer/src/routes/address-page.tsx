import { Link, useParams } from "@tanstack/react-router";
import { useQueryClient } from "@tanstack/react-query";
import { useState, type FormEvent } from "react";
import {
  useAddress,
  useAddressTransfers,
  useAddressTxs,
  useContractMeta,
  useErc20Meta,
  useKnownTokenBalances,
} from "@/hooks/use-chain";
import { formatDew, hexToBigInt, hexToNumber, isHexAddress } from "@/lib/format";
import { formatTokenAmount } from "@/lib/erc20";
import { config } from "@/lib/config";
import { indexerEnabled, registerContract } from "@/lib/indexer";
import { qk } from "@/lib/query-keys";
import { Identicon } from "@/components/identicon";
import {
  AddressLink,
  CopyButton,
  DataField,
  EmptyState,
  FieldList,
  LoadingBlock,
  PageHeader,
  Tabs,
  WarningBanner,
} from "@/components/ui";

export function AddressPage() {
  const { addr } = useParams({ from: "/address/$addr" });
  const valid = isHexAddress(addr);
  const q = useAddress(valid ? addr : "");

  if (!valid) {
    return (
      <EmptyState
        title="Invalid address"
        detail="Expected 0x-prefixed 20-byte hex address."
      />
    );
  }

  if (q.isLoading) return <LoadingBlock label="Loading account…" />;
  if (q.isError) {
    return <EmptyState title="Failed to load address" detail={String(q.error)} />;
  }
  if (!q.data) {
    return <EmptyState title="No data" />;
  }

  const { balance, nonce, code } = q.data;
  const isContract = code != null && code !== "0x" && code !== "0x0";
  const codeBytes = isContract ? Math.max(0, (code.length - 2) / 2) : 0;

  return (
    <AddressBody
      addr={addr}
      balance={balance}
      nonce={nonce}
      code={code}
      isContract={isContract}
      codeBytes={codeBytes}
    />
  );
}

function AddressBody({
  addr,
  balance,
  nonce,
  code,
  isContract,
  codeBytes,
}: {
  addr: string;
  balance: string;
  nonce: string;
  code: string;
  isContract: boolean;
  codeBytes: number;
}) {
  const erc20 = useErc20Meta(addr, isContract);
  const token = erc20.data;
  const knownBal = useKnownTokenBalances(addr);
  const hasKnown = config.knownTokens.length > 0;
  const hasIndexer = indexerEnabled();
  const histTxs = useAddressTxs(addr);
  const histXfer = useAddressTransfers(addr);
  const contractMeta = useContractMeta(addr, isContract);
  const registered = contractMeta.data;

  return (
    <div>
      <div className="mb-6 flex flex-col gap-4 sm:flex-row sm:items-center">
        <Identicon address={addr} size={56} />
        <div className="min-w-0 flex-1">
          <PageHeader
            title={
              <span className="mono break-all text-lg sm:text-2xl">{addr.toLowerCase()}</span>
            }
            badges={
              <span className="flex flex-wrap gap-1.5">
                <span className="rounded-full border border-[var(--color-line)] px-2.5 py-0.5 text-xs text-slate">
                  {isContract ? "Contract" : "EOA"}
                </span>
                {token ? (
                  <span className="rounded-full border border-[var(--ex-status-final-border)] bg-[var(--ex-status-final-bg)] px-2.5 py-0.5 text-xs text-cyan">
                    Token{token.symbol ? ` · ${token.symbol}` : ""}
                  </span>
                ) : null}
                {registered ? (
                  <span
                    className="rounded-full border border-[var(--ex-status-final-border)] bg-[var(--ex-status-final-bg)] px-2.5 py-0.5 text-xs text-cyan"
                    title="ABI submitted to indexer — not solc bytecode match"
                  >
                    ABI registered
                  </span>
                ) : null}
              </span>
            }
            actions={<CopyButton value={addr.toLowerCase()} label="Copy address" />}
          />
          {token?.name || token?.symbol ? (
            <p className="mt-1 text-sm text-slate">
              {token.name ?? "Token"}
              {token.symbol ? (
                <span className="text-muted"> ({token.symbol})</span>
              ) : null}
            </p>
          ) : null}
          <p className="text-2xl font-semibold text-frost sm:text-3xl">
            {formatDew(hexToBigInt(balance))}{" "}
            <span className="text-base font-normal text-muted">{config.symbol}</span>
          </p>
          <p className="mono mt-1 text-xs text-muted">{hexToBigInt(balance).toString()} wei</p>
        </div>
      </div>

      <WarningBanner>
        {hasIndexer
          ? "History and token transfers load from the optional indexer sidecar when caught up."
          : "Full address history needs an indexer (set PUBLIC_INDEXER_URL). Showing on-chain balance, nonce, and code from JSON-RPC"}
        {!hasIndexer && token ? "; ERC-20 metadata via eth_call" : ""}
        {!hasIndexer && hasKnown ? "; known-token balances via balanceOf" : ""}
        {!hasIndexer ? "." : null}
      </WarningBanner>

      <Tabs.Root defaultValue="overview">
        <Tabs.List className="mb-4 flex gap-1 border-b border-[var(--color-line)]">
          <Tabs.Trigger value="overview" className="tab-trigger">
            Overview
          </Tabs.Trigger>
          {hasIndexer ? (
            <Tabs.Trigger value="history" className="tab-trigger">
              Transactions
              {histTxs.data ? ` (${histTxs.data.length})` : ""}
            </Tabs.Trigger>
          ) : null}
          {hasIndexer ? (
            <Tabs.Trigger value="transfers" className="tab-trigger">
              Token transfers
              {histXfer.data ? ` (${histXfer.data.length})` : ""}
            </Tabs.Trigger>
          ) : null}
          {hasKnown ? (
            <Tabs.Trigger value="tokens" className="tab-trigger">
              Token balances
              {knownBal.data
                ? ` (${knownBal.data.filter((r) => r.balance > 0n).length})`
                : ""}
            </Tabs.Trigger>
          ) : null}
          {token ? (
            <Tabs.Trigger value="token" className="tab-trigger">
              Token
            </Tabs.Trigger>
          ) : null}
          {isContract ? (
            <Tabs.Trigger value="contract" className="tab-trigger">
              Contract
            </Tabs.Trigger>
          ) : null}
        </Tabs.List>

        <Tabs.Content value="overview">
          <FieldList>
            <DataField label="Balance">
              <span className="mono">
                {formatDew(hexToBigInt(balance))} {config.symbol}
              </span>
            </DataField>
            <DataField label="Nonce">{hexToNumber(nonce)}</DataField>
            <DataField label="Account type">{isContract ? "Contract" : "EOA"}</DataField>
            {isContract ? (
              <DataField label="Code size">{codeBytes.toLocaleString()} bytes</DataField>
            ) : null}
            {token?.symbol ? (
              <DataField label="Detected token">
                <span className="text-sm">
                  {token.name ?? "—"}{" "}
                  <span className="mono text-cyan">{token.symbol}</span>
                </span>
              </DataField>
            ) : null}
          </FieldList>
        </Tabs.Content>

        {hasIndexer ? (
          <Tabs.Content value="history">
            {histTxs.isLoading ? (
              <LoadingBlock label="Loading transaction history…" />
            ) : histTxs.isError ? (
              <EmptyState title="Indexer unavailable" detail={String(histTxs.error)} />
            ) : !histTxs.data?.length ? (
              <EmptyState
                title="No indexed transactions"
                detail="Indexer may still be catching up, or this address has no txs yet."
              />
            ) : (
              <div className="overflow-x-auto">
                <table className="w-full min-w-[32rem] text-left text-sm">
                  <thead>
                    <tr className="border-b border-[var(--color-line)] text-xs text-muted">
                      <th className="pb-2 pr-3 font-medium">Tx</th>
                      <th className="pb-2 pr-3 font-medium">Block</th>
                      <th className="pb-2 pr-3 font-medium">From / To</th>
                      <th className="pb-2 text-right font-medium">Value</th>
                    </tr>
                  </thead>
                  <tbody>
                    {histTxs.data.map((tx) => (
                      <tr
                        key={tx.hash}
                        className="border-b border-[var(--color-line)]/60 last:border-0"
                      >
                        <td className="py-2.5 pr-3 mono">
                          <Link
                            to="/tx/$hash"
                            params={{ hash: tx.hash }}
                            className="text-cyan no-underline hover:underline"
                          >
                            {tx.hash.slice(0, 10)}…
                          </Link>
                        </td>
                        <td className="py-2.5 pr-3 mono">
                          <Link
                            to="/block/$id"
                            params={{ id: String(tx.blockNumber) }}
                            className="no-underline hover:underline"
                          >
                            {tx.blockNumber}
                          </Link>
                        </td>
                        <td className="py-2.5 pr-3 text-xs">
                          <div>
                            <span className="text-muted">from </span>
                            <AddressLink address={tx.from} />
                          </div>
                          {tx.to ? (
                            <div>
                              <span className="text-muted">to </span>
                              <AddressLink address={tx.to} />
                            </div>
                          ) : (
                            <span className="text-muted">contract create</span>
                          )}
                        </td>
                        <td className="py-2.5 text-right mono text-xs">
                          {formatDew(hexToBigInt(tx.value || "0x0"))}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </Tabs.Content>
        ) : null}

        {hasIndexer ? (
          <Tabs.Content value="transfers">
            {histXfer.isLoading ? (
              <LoadingBlock label="Loading token transfers…" />
            ) : histXfer.isError ? (
              <EmptyState title="Indexer unavailable" detail={String(histXfer.error)} />
            ) : !histXfer.data?.length ? (
              <EmptyState title="No ERC-20 transfers" detail="No Transfer events for this address." />
            ) : (
              <div className="overflow-x-auto">
                <table className="w-full min-w-[32rem] text-left text-sm">
                  <thead>
                    <tr className="border-b border-[var(--color-line)] text-xs text-muted">
                      <th className="pb-2 pr-3 font-medium">Tx</th>
                      <th className="pb-2 pr-3 font-medium">Token</th>
                      <th className="pb-2 pr-3 font-medium">From → To</th>
                      <th className="pb-2 text-right font-medium">Amount</th>
                    </tr>
                  </thead>
                  <tbody>
                    {histXfer.data.map((xf) => (
                      <tr
                        key={`${xf.txHash}-${xf.logIndex}`}
                        className="border-b border-[var(--color-line)]/60 last:border-0"
                      >
                        <td className="py-2.5 pr-3 mono">
                          <Link
                            to="/tx/$hash"
                            params={{ hash: xf.txHash }}
                            className="text-cyan no-underline hover:underline"
                          >
                            {xf.txHash.slice(0, 10)}…
                          </Link>
                        </td>
                        <td className="py-2.5 pr-3">
                          <AddressLink address={xf.token} />
                        </td>
                        <td className="py-2.5 pr-3 text-xs">
                          <AddressLink address={xf.from} /> → <AddressLink address={xf.to} />
                        </td>
                        <td className="py-2.5 text-right mono text-xs">
                          {hexToBigInt(xf.amount || "0x0").toString()}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </Tabs.Content>
        ) : null}

        {hasKnown ? (
          <Tabs.Content value="tokens">
            {knownBal.isLoading ? (
              <LoadingBlock label="Loading token balances…" />
            ) : knownBal.isError ? (
              <EmptyState title="Failed to load token balances" detail={String(knownBal.error)} />
            ) : !knownBal.data?.length ? (
              <EmptyState
                title="No known tokens configured"
                detail="Set PUBLIC_KNOWN_TOKENS (e.g. SYMBOL:0xaddr,0x…) at build time."
              />
            ) : (
              <div className="overflow-x-auto">
                <table className="w-full min-w-[28rem] text-left text-sm">
                  <thead>
                    <tr className="border-b border-[var(--color-line)] text-xs text-muted">
                      <th className="pb-2 pr-3 font-medium">Token</th>
                      <th className="pb-2 pr-3 font-medium">Contract</th>
                      <th className="pb-2 text-right font-medium">Balance</th>
                    </tr>
                  </thead>
                  <tbody>
                    {knownBal.data.map((row) => (
                      <tr
                        key={row.address}
                        className="border-b border-[var(--color-line)]/60 last:border-0"
                      >
                        <td className="py-2.5 pr-3">
                          <span className="font-medium text-frost">
                            {row.symbol ?? row.label ?? "Token"}
                          </span>
                          {row.error ? (
                            <span className="mt-0.5 block text-[10px] text-danger">{row.error}</span>
                          ) : null}
                        </td>
                        <td className="py-2.5 pr-3">
                          <AddressLink address={row.address} />
                        </td>
                        <td className="mono py-2.5 text-right text-frost">
                          {formatTokenAmount(row.balance, row.decimals)}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
                <p className="mt-3 text-xs text-muted">
                  List from <span className="mono">PUBLIC_KNOWN_TOKENS</span> — not a full
                  portfolio indexer.
                </p>
              </div>
            )}
          </Tabs.Content>
        ) : null}

        {token ? (
          <Tabs.Content value="token">
            <FieldList>
              <DataField label="Name">{token.name ?? "—"}</DataField>
              <DataField label="Symbol">
                <span className="mono">{token.symbol ?? "—"}</span>
              </DataField>
              <DataField label="Decimals">
                {token.decimals != null ? token.decimals : "—"}
              </DataField>
              <DataField label="Total supply">
                {token.totalSupply != null ? (
                  <span className="mono">
                    {formatTokenAmount(token.totalSupply, token.decimals)}
                    {token.symbol ? ` ${token.symbol}` : ""}
                    <span className="mt-0.5 block text-xs text-muted">
                      {token.totalSupply.toString()} raw
                    </span>
                  </span>
                ) : (
                  "—"
                )}
              </DataField>
            </FieldList>
          </Tabs.Content>
        ) : null}

        {isContract ? (
          <Tabs.Content value="contract">
            <ContractTab
              addr={addr}
              code={code}
              hasIndexer={hasIndexer}
              metaLoading={contractMeta.isLoading}
              metaError={contractMeta.isError ? String(contractMeta.error) : null}
              meta={registered ?? null}
            />
          </Tabs.Content>
        ) : null}
      </Tabs.Root>
    </div>
  );
}

function ContractTab({
  addr,
  code,
  hasIndexer,
  metaLoading,
  metaError,
  meta,
}: {
  addr: string;
  code: string;
  hasIndexer: boolean;
  metaLoading: boolean;
  metaError: string | null;
  meta: {
    name?: string;
    abi: unknown[];
    source?: string;
    compiler?: string;
    status: string;
  } | null;
}) {
  const qc = useQueryClient();
  const [name, setName] = useState("");
  const [compiler, setCompiler] = useState("");
  const [abiText, setAbiText] = useState("[]");
  const [source, setSource] = useState("");
  const [busy, setBusy] = useState(false);
  const [formError, setFormError] = useState<string | null>(null);
  const [okMsg, setOkMsg] = useState<string | null>(null);

  async function onRegister(e: FormEvent) {
    e.preventDefault();
    setFormError(null);
    setOkMsg(null);
    let abi: unknown[];
    try {
      const parsed = JSON.parse(abiText) as unknown;
      if (!Array.isArray(parsed)) {
        setFormError("ABI must be a JSON array");
        return;
      }
      abi = parsed;
    } catch {
      setFormError("ABI is not valid JSON");
      return;
    }
    setBusy(true);
    try {
      await registerContract(addr, {
        name: name.trim() || undefined,
        compiler: compiler.trim() || undefined,
        abi,
        source: source.trim() || undefined,
      });
      await qc.invalidateQueries({ queryKey: qk.contractMeta(addr) });
      setOkMsg("ABI registered");
    } catch (err) {
      setFormError(String(err));
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="flex flex-col gap-6">
      {!hasIndexer ? (
        <p className="text-sm text-muted">
          Set <span className="mono">PUBLIC_INDEXER_URL</span> to load or register ABI/source
          (P1f). Bytecode below is from JSON-RPC only.
        </p>
      ) : null}

      {hasIndexer && metaLoading ? <LoadingBlock label="Loading contract registry…" /> : null}
      {hasIndexer && metaError ? (
        <EmptyState title="Registry unavailable" detail={metaError} />
      ) : null}

      {meta ? (
        <FieldList>
          <DataField label="Registry">
            <span className="text-sm text-cyan">ABI registered</span>
            <span className="mt-1 block text-xs text-muted">
              Not a solc bytecode match — submitted metadata only.
            </span>
          </DataField>
          {meta.name ? <DataField label="Name">{meta.name}</DataField> : null}
          {meta.compiler ? (
            <DataField label="Compiler">
              <span className="mono text-sm">{meta.compiler}</span>
            </DataField>
          ) : null}
          <DataField label="ABI">
            <pre className="mono surface-md max-h-72 overflow-auto whitespace-pre-wrap break-all border border-[var(--color-line)] bg-ink-soft p-3 text-xs text-frost">
              {JSON.stringify(meta.abi, null, 2)}
            </pre>
          </DataField>
          {meta.source ? (
            <DataField label="Source">
              <pre className="mono surface-md max-h-96 overflow-auto whitespace-pre-wrap break-all border border-[var(--color-line)] bg-ink-soft p-3 text-xs text-frost">
                {meta.source}
              </pre>
            </DataField>
          ) : null}
        </FieldList>
      ) : null}

      {hasIndexer && !metaLoading && !meta ? (
        <form onSubmit={onRegister} className="flex flex-col gap-3">
          <p className="text-sm text-slate">
            Register ABI (and optional source) for this address. Badge shows{" "}
            <strong className="text-frost">ABI registered</strong> — not verified-by-compiler.
          </p>
          <label className="flex flex-col gap-1 text-xs text-muted">
            Name (optional)
            <input
              className="mono rounded border border-[var(--color-line)] bg-ink-soft px-3 py-2 text-sm text-frost"
              value={name}
              onChange={(e) => setName(e.target.value)}
              maxLength={128}
            />
          </label>
          <label className="flex flex-col gap-1 text-xs text-muted">
            Compiler (optional)
            <input
              className="mono rounded border border-[var(--color-line)] bg-ink-soft px-3 py-2 text-sm text-frost"
              value={compiler}
              onChange={(e) => setCompiler(e.target.value)}
              placeholder="solc 0.8.24"
              maxLength={64}
            />
          </label>
          <label className="flex flex-col gap-1 text-xs text-muted">
            ABI JSON array (required)
            <textarea
              className="mono min-h-[8rem] rounded border border-[var(--color-line)] bg-ink-soft px-3 py-2 text-xs text-frost"
              value={abiText}
              onChange={(e) => setAbiText(e.target.value)}
              spellCheck={false}
            />
          </label>
          <label className="flex flex-col gap-1 text-xs text-muted">
            Source (optional)
            <textarea
              className="mono min-h-[6rem] rounded border border-[var(--color-line)] bg-ink-soft px-3 py-2 text-xs text-frost"
              value={source}
              onChange={(e) => setSource(e.target.value)}
              spellCheck={false}
            />
          </label>
          {formError ? <p className="text-sm text-rose-400">{formError}</p> : null}
          {okMsg ? <p className="text-sm text-cyan">{okMsg}</p> : null}
          <button
            type="submit"
            disabled={busy}
            className="w-fit rounded border border-[var(--color-line)] bg-ink-soft px-4 py-2 text-sm text-frost hover:border-cyan disabled:opacity-50"
          >
            {busy ? "Submitting…" : "Register ABI"}
          </button>
        </form>
      ) : null}

      <FieldList>
        <DataField label="Bytecode">
          <pre className="mono surface-md max-h-96 overflow-auto whitespace-pre-wrap break-all border border-[var(--color-line)] bg-ink-soft p-3 text-xs text-frost">
            {code}
          </pre>
        </DataField>
      </FieldList>
    </div>
  );
}
