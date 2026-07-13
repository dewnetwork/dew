import { IconCode, IconTerminal2 } from '@tabler/icons-react';
import { useState } from 'react';
import { PUBLIC_NETWORK } from '../../lib/network';

type Tab = 'public' | 'local';

const tabs: { id: Tab; label: string; subtitle: string }[] = [
  { id: 'public', label: 'Public', subtitle: `chain ${PUBLIC_NETWORK.chainId}` },
  { id: 'local', label: 'Local', subtitle: 'devnet :8545' },
];

function PublicSnippet() {
  return (
    <code className="block">
      <span className="text-muted">
        # Public testnet ({PUBLIC_NETWORK.freezeTag})
      </span>
      {'\n'}
      <span className="text-teal">export</span>
      {` ETH_RPC_URL=${PUBLIC_NETWORK.rpc}`}
      {'\n\n'}
      <span className="text-muted"># Use a faucet-funded key — never Anvil #0</span>
      {'\n'}
      {`forge create src/Counter.sol:Counter \\
  --rpc-url $ETH_RPC_URL \\
  --private-key $DEPLOYER_KEY \\
  --broadcast`}
      {'\n\n'}
      <span className="text-muted"># Finality: one commit, not N confirmations</span>
      {'\n'}
      {`cast receipt $TX_HASH --rpc-url $ETH_RPC_URL`}
    </code>
  );
}

function LocalSnippet() {
  return (
    <code className="block">
      <span className="text-muted"># Local multi-validator devnet</span>
      {'\n'}
      {`go build -o bin/dew ./cmd/dew
./bin/dew devnet --http.port 8545`}
      {'\n\n'}
      <span className="text-teal">export</span>
      {' ETH_RPC_URL=http://127.0.0.1:8545'}
      {'\n'}
      <span className="text-teal">export</span>
      {
        ' PRIVATE_KEY=0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80'
      }
      {'\n\n'}
      {`forge create src/Counter.sol:Counter \\
  --rpc-url $ETH_RPC_URL \\
  --private-key $PRIVATE_KEY \\
  --broadcast`}
      {'\n\n'}
      <span className="text-muted"># Anvil #0 is OK on local only</span>
      {'\n'}
      {`cast receipt $TX_HASH --rpc-url $ETH_RPC_URL`}
    </code>
  );
}

export function BuilderTerminal() {
  const [tab, setTab] = useState<Tab>('public');

  return (
    <div className="terminal-frame flex h-full min-h-full flex-col overflow-hidden rounded-2xl border border-[var(--color-line)] bg-ink shadow-[var(--shadow-panel)]">
      <div className="flex shrink-0 items-center gap-2 border-b border-[var(--color-line)] bg-panel/40 px-3 py-2.5 sm:px-4">
        <span className="h-2.5 w-2.5 shrink-0 rounded-full bg-[#ff5f57]" />
        <span className="h-2.5 w-2.5 shrink-0 rounded-full bg-[#febc2e]" />
        <span className="h-2.5 w-2.5 shrink-0 rounded-full bg-[#28c840]" />
        <span className="ml-1 hidden min-w-0 items-center gap-1.5 truncate font-mono text-[11px] tracking-wide text-muted sm:inline-flex">
          <IconTerminal2 size={13} stroke={1.75} aria-hidden />
          foundry
        </span>
        <div
          className="ml-auto flex rounded-lg border border-[var(--color-line)] bg-ink/60 p-0.5"
          role="tablist"
          aria-label="RPC target"
        >
          {tabs.map((t) => {
            const active = tab === t.id;
            return (
              <button
                key={t.id}
                type="button"
                role="tab"
                aria-selected={active}
                id={`builders-tab-${t.id}`}
                aria-controls={`builders-panel-${t.id}`}
                className={`rounded-md px-2.5 py-1 font-mono text-[11px] tracking-wide transition ${
                  active
                    ? 'bg-panel text-frost shadow-[0_0_0_1px_rgb(94_234_212_/_0.2)]'
                    : 'text-muted hover:text-mist'
                }`}
                onClick={() => setTab(t.id)}
              >
                {t.label}
                <span className="ml-1 hidden text-[10px] opacity-70 sm:inline">
                  {t.subtitle}
                </span>
              </button>
            );
          })}
        </div>
      </div>

      <div
        className="flex min-h-0 flex-1 flex-col justify-center overflow-x-auto"
        role="tabpanel"
        id={`builders-panel-${tab}`}
        aria-labelledby={`builders-tab-${tab}`}
      >
        <pre className="m-0 p-5 font-mono text-[12.5px] leading-[1.7] whitespace-pre text-mist sm:p-6 sm:text-[13px]">
          {tab === 'public' ? <PublicSnippet /> : <LocalSnippet />}
        </pre>
      </div>

      <div className="flex shrink-0 items-start gap-2 border-t border-[var(--color-line)] bg-panel/40 px-5 py-3.5 font-mono text-[11px] leading-snug text-muted">
        <IconCode
          size={14}
          stroke={1.75}
          className="mt-0.5 shrink-0 text-cyan"
          aria-hidden
        />
        <span>
          {tab === 'public'
            ? 'JSON-RPC eth_* · faucet key only · never Anvil defaults on public nets'
            : 'JSON-RPC eth_* · Anvil #0 OK locally · chain 2205'}
        </span>
      </div>
    </div>
  );
}
