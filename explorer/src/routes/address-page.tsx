import { useParams } from "@tanstack/react-router";
import { useAddress } from "@/hooks/use-chain";
import { formatDew, hexToBigInt, hexToNumber, isHexAddress } from "@/lib/format";
import { config } from "@/lib/config";
import { Identicon } from "@/components/identicon";
import {
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
    <div>
      <div className="mb-6 flex flex-col gap-4 sm:flex-row sm:items-center">
        <Identicon address={addr} size={56} />
        <div className="min-w-0 flex-1">
          <PageHeader
            title={
              <span className="mono break-all text-lg sm:text-2xl">{addr.toLowerCase()}</span>
            }
            badges={
              <span className="rounded-full border border-[var(--color-line)] px-2.5 py-0.5 text-xs text-slate">
                {isContract ? "Contract" : "EOA"}
              </span>
            }
            actions={<CopyButton value={addr.toLowerCase()} label="Copy address" />}
          />
          <p className="text-2xl font-semibold text-frost sm:text-3xl">
            {formatDew(hexToBigInt(balance))}{" "}
            <span className="text-base font-normal text-muted">{config.symbol}</span>
          </p>
          <p className="mono mt-1 text-xs text-muted">{hexToBigInt(balance).toString()} wei</p>
        </div>
      </div>

      <WarningBanner>
        Full address history needs an indexer. Showing on-chain balance, nonce, and code from
        JSON-RPC.
      </WarningBanner>

      <Tabs.Root defaultValue="overview">
        <Tabs.List className="mb-4 flex gap-1 border-b border-[var(--color-line)]">
          <Tabs.Trigger value="overview" className="tab-trigger">
            Overview
          </Tabs.Trigger>
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
          </FieldList>
        </Tabs.Content>

        {isContract ? (
          <Tabs.Content value="contract">
            <FieldList>
              <DataField label="Bytecode">
                <pre className="mono surface-md max-h-96 overflow-auto whitespace-pre-wrap break-all border border-[var(--color-line)] bg-ink-soft p-3 text-xs text-frost">
                  {code}
                </pre>
              </DataField>
            </FieldList>
          </Tabs.Content>
        ) : null}
      </Tabs.Root>
    </div>
  );
}
