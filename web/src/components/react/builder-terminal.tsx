import { IconCode, IconTerminal2 } from '@tabler/icons-react';
import { Fragment, useState } from 'react';
import { PUBLIC_NETWORK } from '../../lib/network';

type Tab = 'public' | 'local';

const tabs: { id: Tab; label: string; subtitle: string }[] = [
  { id: 'public', label: 'Public', subtitle: `chain ${PUBLIC_NETWORK.chainId}` },
  { id: 'local', label: 'Local', subtitle: 'devnet :8545' },
];

/** One visual line in the faux shell. */
type ShellLine =
  | { kind: 'blank' }
  | { kind: 'comment'; text: string }
  | { kind: 'cmd'; parts: ShellPart[] };

type ShellPart =
  | { t: 'prompt' }
  | { t: 'bin'; text: string }
  | { t: 'kw'; text: string }
  | { t: 'flag'; text: string }
  | { t: 'arg'; text: string }
  | { t: 'str'; text: string }
  | { t: 'var'; text: string }
  | { t: 'op'; text: string }
  | { t: 'plain'; text: string };

const partClass: Record<ShellPart['t'], string> = {
  prompt: 'select-none text-cyan/70',
  bin: 'font-medium text-frost',
  kw: 'text-teal',
  flag: 'text-mist/80',
  arg: 'text-mist',
  str: 'text-gold',
  var: 'text-cyan',
  op: 'text-muted',
  plain: 'text-mist',
};

function Part({ p }: { p: ShellPart }) {
  if (p.t === 'prompt') {
    return <span className={partClass.prompt}>$ </span>;
  }
  return <span className={partClass[p.t]}>{p.text}</span>;
}

function ShellBlock({ lines }: { lines: ShellLine[] }) {
  return (
    <code className="block">
      {lines.map((line, i) => {
        if (line.kind === 'blank') {
          return <Fragment key={i}>{'\n'}</Fragment>;
        }
        if (line.kind === 'comment') {
          return (
            <Fragment key={i}>
              {i > 0 ? '\n' : null}
              <span className="text-muted">{line.text}</span>
            </Fragment>
          );
        }
        return (
          <Fragment key={i}>
            {i > 0 ? '\n' : null}
            {line.parts.map((p, j) => (
              <Part key={j} p={p} />
            ))}
          </Fragment>
        );
      })}
    </code>
  );
}

const publicLines: ShellLine[] = [
  { kind: 'comment', text: `# ${PUBLIC_NETWORK.freezeTag} · chain ${PUBLIC_NETWORK.chainId}` },
  {
    kind: 'cmd',
    parts: [
      { t: 'prompt' },
      { t: 'kw', text: 'export' },
      { t: 'plain', text: ' ' },
      { t: 'var', text: 'ETH_RPC_URL' },
      { t: 'op', text: '=' },
      { t: 'str', text: PUBLIC_NETWORK.rpc },
    ],
  },
  { kind: 'blank' },
  { kind: 'comment', text: '# faucet-funded key only — never Anvil #0' },
  {
    kind: 'cmd',
    parts: [
      { t: 'prompt' },
      { t: 'bin', text: 'forge' },
      { t: 'plain', text: ' ' },
      { t: 'arg', text: 'create' },
      { t: 'plain', text: ' ' },
      { t: 'str', text: 'src/Counter.sol:Counter' },
      { t: 'op', text: ' \\' },
    ],
  },
  {
    kind: 'cmd',
    parts: [
      { t: 'plain', text: '    ' },
      { t: 'flag', text: '--rpc-url' },
      { t: 'plain', text: ' ' },
      { t: 'var', text: '$ETH_RPC_URL' },
      { t: 'op', text: ' \\' },
    ],
  },
  {
    kind: 'cmd',
    parts: [
      { t: 'plain', text: '    ' },
      { t: 'flag', text: '--private-key' },
      { t: 'plain', text: ' ' },
      { t: 'var', text: '$DEPLOYER_KEY' },
      { t: 'op', text: ' \\' },
    ],
  },
  {
    kind: 'cmd',
    parts: [
      { t: 'plain', text: '    ' },
      { t: 'flag', text: '--broadcast' },
    ],
  },
  { kind: 'blank' },
  { kind: 'comment', text: '# finality: one commit' },
  {
    kind: 'cmd',
    parts: [
      { t: 'prompt' },
      { t: 'bin', text: 'cast' },
      { t: 'plain', text: ' ' },
      { t: 'arg', text: 'receipt' },
      { t: 'plain', text: ' ' },
      { t: 'var', text: '$TX_HASH' },
      { t: 'plain', text: ' ' },
      { t: 'flag', text: '--rpc-url' },
      { t: 'plain', text: ' ' },
      { t: 'var', text: '$ETH_RPC_URL' },
    ],
  },
];

const localLines: ShellLine[] = [
  { kind: 'comment', text: '# multi-validator local devnet' },
  {
    kind: 'cmd',
    parts: [
      { t: 'prompt' },
      { t: 'bin', text: 'go' },
      { t: 'plain', text: ' ' },
      { t: 'arg', text: 'build' },
      { t: 'plain', text: ' ' },
      { t: 'flag', text: '-o' },
      { t: 'plain', text: ' ' },
      { t: 'str', text: 'bin/dew' },
      { t: 'plain', text: ' ' },
      { t: 'arg', text: './cmd/dew' },
    ],
  },
  {
    kind: 'cmd',
    parts: [
      { t: 'prompt' },
      { t: 'bin', text: './bin/dew' },
      { t: 'plain', text: ' ' },
      { t: 'arg', text: 'devnet' },
      { t: 'plain', text: ' ' },
      { t: 'flag', text: '--http.port' },
      { t: 'plain', text: ' ' },
      { t: 'str', text: '8545' },
    ],
  },
  { kind: 'blank' },
  {
    kind: 'cmd',
    parts: [
      { t: 'prompt' },
      { t: 'kw', text: 'export' },
      { t: 'plain', text: ' ' },
      { t: 'var', text: 'ETH_RPC_URL' },
      { t: 'op', text: '=' },
      { t: 'str', text: 'http://127.0.0.1:8545' },
    ],
  },
  {
    kind: 'cmd',
    parts: [
      { t: 'prompt' },
      { t: 'kw', text: 'export' },
      { t: 'plain', text: ' ' },
      { t: 'var', text: 'PRIVATE_KEY' },
      { t: 'op', text: '=' },
      { t: 'str', text: '0xac09…ff80' },
    ],
  },
  { kind: 'blank' },
  {
    kind: 'cmd',
    parts: [
      { t: 'prompt' },
      { t: 'bin', text: 'forge' },
      { t: 'plain', text: ' ' },
      { t: 'arg', text: 'create' },
      { t: 'plain', text: ' ' },
      { t: 'str', text: 'src/Counter.sol:Counter' },
      { t: 'op', text: ' \\' },
    ],
  },
  {
    kind: 'cmd',
    parts: [
      { t: 'plain', text: '    ' },
      { t: 'flag', text: '--rpc-url' },
      { t: 'plain', text: ' ' },
      { t: 'var', text: '$ETH_RPC_URL' },
      { t: 'op', text: ' \\' },
    ],
  },
  {
    kind: 'cmd',
    parts: [
      { t: 'plain', text: '    ' },
      { t: 'flag', text: '--private-key' },
      { t: 'plain', text: ' ' },
      { t: 'var', text: '$PRIVATE_KEY' },
      { t: 'op', text: ' \\' },
    ],
  },
  {
    kind: 'cmd',
    parts: [
      { t: 'plain', text: '    ' },
      { t: 'flag', text: '--broadcast' },
    ],
  },
  { kind: 'blank' },
  { kind: 'comment', text: '# Anvil #0 OK on local only' },
  {
    kind: 'cmd',
    parts: [
      { t: 'prompt' },
      { t: 'bin', text: 'cast' },
      { t: 'plain', text: ' ' },
      { t: 'arg', text: 'receipt' },
      { t: 'plain', text: ' ' },
      { t: 'var', text: '$TX_HASH' },
      { t: 'plain', text: ' ' },
      { t: 'flag', text: '--rpc-url' },
      { t: 'plain', text: ' ' },
      { t: 'var', text: '$ETH_RPC_URL' },
    ],
  },
];

export function BuilderTerminal() {
  const [tab, setTab] = useState<Tab>('public');
  const lines = tab === 'public' ? publicLines : localLines;

  return (
    <div className="terminal-frame flex h-full min-h-full flex-col overflow-hidden rounded-2xl border border-[var(--color-line)] bg-ink shadow-[var(--shadow-panel)]">
      <div className="flex shrink-0 items-center gap-2 border-b border-[var(--color-line)] bg-panel/40 px-3 py-2.5 sm:px-4">
        <span className="h-2.5 w-2.5 shrink-0 rounded-full bg-[#ff5f57]" />
        <span className="h-2.5 w-2.5 shrink-0 rounded-full bg-[#febc2e]" />
        <span className="h-2.5 w-2.5 shrink-0 rounded-full bg-[#28c840]" />
        <span className="ml-1 hidden min-w-0 items-center gap-1.5 truncate font-mono text-[11px] tracking-wide text-muted sm:inline-flex">
          <IconTerminal2 size={13} stroke={1.75} aria-hidden />
          zsh · foundry
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
        <pre className="m-0 p-5 font-mono text-[12.5px] leading-[1.75] whitespace-pre sm:p-6 sm:text-[13px]">
          <ShellBlock lines={lines} />
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
