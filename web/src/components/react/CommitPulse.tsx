import {
  IconActivity,
  IconCircleCheck,
  IconClockHour4,
  IconStack2,
} from '@tabler/icons-react';
import { AnimatePresence, motion, useReducedMotion } from 'motion/react';
import { useEffect, useState } from 'react';

type Phase = 'propose' | 'prevote' | 'precommit' | 'commit';

const PHASES: Phase[] = ['propose', 'prevote', 'precommit', 'commit'];
const PHASE_MS = 250;
const HOLD_MS = 280;
const CYCLE_MS = PHASES.length * PHASE_MS + HOLD_MS;

const LABELS: Record<Phase, string> = {
  propose: 'PROPOSE',
  prevote: 'PREVOTE',
  precommit: 'PRECOMMIT',
  commit: 'COMMIT',
};

function padHeight(n: number) {
  return n.toString().padStart(7, '0');
}

export function CommitPulse() {
  const reduce = useReducedMotion();
  const [height, setHeight] = useState(1_048_576);
  const [phaseIdx, setPhaseIdx] = useState(0);
  const [history, setHistory] = useState<number[]>(() =>
    Array.from({ length: 5 }, (_, i) => 1_048_575 - i),
  );

  useEffect(() => {
    if (reduce) return;

    let cancelled = false;
    let timer: ReturnType<typeof setTimeout>;

    const run = () => {
      let i = 0;
      const step = () => {
        if (cancelled) return;
        setPhaseIdx(i);
        if (i < PHASES.length - 1) {
          i += 1;
          timer = setTimeout(step, PHASE_MS);
        } else {
          timer = setTimeout(() => {
            if (cancelled) return;
            setHeight((h) => {
              const next = h + 1;
              setHistory((prev) => [h, ...prev].slice(0, 6));
              return next;
            });
            setPhaseIdx(0);
            timer = setTimeout(run, 40);
          }, HOLD_MS);
        }
      };
      step();
    };

    timer = setTimeout(run, 600);
    return () => {
      cancelled = true;
      clearTimeout(timer);
    };
  }, [reduce]);

  const phase = PHASES[phaseIdx];
  const isCommit = phase === 'commit';

  return (
    <div
      className="relative w-full max-w-md overflow-hidden rounded-2xl border border-[var(--color-line)] bg-ink-soft/90 shadow-[var(--shadow-panel)] backdrop-blur-sm"
      role="img"
      aria-label="Dew-BFT commit visualization: blocks finalize about once per second"
    >
      <div
        className="pointer-events-none absolute -right-16 -top-20 h-48 w-48 rounded-full bg-cyan/20 blur-3xl transition-opacity duration-500"
        style={{ opacity: isCommit ? 0.55 : 0.2 }}
        aria-hidden
      />
      <div
        className="pointer-events-none absolute -bottom-12 -left-10 h-36 w-36 rounded-full bg-teal/15 blur-3xl"
        aria-hidden
      />

      <div className="relative border-b border-[var(--color-line)] px-5 py-4">
        <div className="flex items-center justify-between gap-3">
          <div className="flex items-center gap-2">
            <span className="inline-flex h-7 w-7 items-center justify-center rounded-lg border border-[var(--color-line)] bg-panel/80 text-cyan">
              <IconActivity size={14} stroke={1.75} />
            </span>
            <span className="font-mono text-[11px] font-medium tracking-[0.14em] text-mist uppercase">
              Dew-BFT · live
            </span>
          </div>
          <span className="inline-flex items-center gap-1 font-mono text-[11px] text-muted">
            <IconClockHour4 size={13} stroke={1.75} />
            ~1s finality
          </span>
        </div>
      </div>

      <div className="relative grid gap-5 p-5 sm:grid-cols-[1fr_auto] sm:items-end">
        <div>
          <p className="inline-flex items-center gap-1.5 font-mono text-[11px] tracking-wider text-muted uppercase">
            <IconStack2 size={13} stroke={1.75} className="text-cyan" />
            Height
          </p>
          <p className="mt-1 font-display text-4xl font-bold tracking-tight text-frost tabular-nums sm:text-5xl">
            {padHeight(height)}
          </p>
          <div className="mt-4 flex flex-wrap gap-1.5">
            {PHASES.map((p) => {
              const active = p === phase;
              const done =
                PHASES.indexOf(p) < phaseIdx || (isCommit && p === 'commit');
              return (
                <span
                  key={p}
                  className={[
                    'rounded-md px-2 py-1 font-mono text-[10px] tracking-wide transition-colors duration-200',
                    active
                      ? isCommit
                        ? 'bg-cyan text-ink'
                        : 'bg-cyan/20 text-cyan'
                      : done
                        ? 'bg-panel text-mist/80'
                        : 'bg-panel/50 text-muted',
                  ].join(' ')}
                >
                  {LABELS[p]}
                </span>
              );
            })}
          </div>
        </div>

        <div className="flex flex-col items-end gap-1">
          <span className="font-mono text-[10px] tracking-wider text-muted uppercase">
            Status
          </span>
          <AnimatePresence mode="wait">
            <motion.span
              key={phase}
              initial={reduce ? false : { opacity: 0, y: 6 }}
              animate={{ opacity: 1, y: 0 }}
              exit={reduce ? undefined : { opacity: 0, y: -6 }}
              transition={{ duration: 0.2 }}
              className={`inline-flex items-center gap-1 font-mono text-sm font-medium tracking-wide ${
                isCommit ? 'text-gold' : 'text-cyan'
              }`}
            >
              {isCommit && <IconCircleCheck size={15} stroke={1.75} />}
              {isCommit ? 'FINAL' : LABELS[phase]}
            </motion.span>
          </AnimatePresence>
        </div>
      </div>

      <div className="relative border-t border-[var(--color-line)] px-5 py-4">
        <div className="mb-3 flex items-center justify-between">
          <span className="font-mono text-[10px] tracking-wider text-muted uppercase">
            Recent commits
          </span>
          <span className="font-mono text-[10px] text-muted">
            cycle {CYCLE_MS}ms
          </span>
        </div>
        <ul className="m-0 list-none space-y-1.5 p-0">
          {history.map((h, i) => (
            <li
              key={h}
              className="m-0 flex list-none items-center gap-3 rounded-lg border border-[var(--color-line)] bg-panel/40 px-3 py-2"
              style={{ opacity: 1 - i * 0.12 }}
            >
              <IconCircleCheck
                size={14}
                stroke={1.75}
                className={
                  i === 0 && isCommit ? 'text-gold' : 'text-cyan/70'
                }
              />
              <span className="font-mono text-xs text-slate tabular-nums">
                #{padHeight(h)}
              </span>
              <span className="ml-auto font-mono text-[10px] text-muted">
                final
              </span>
            </li>
          ))}
        </ul>
      </div>

      <AnimatePresence>
        {isCommit && !reduce && (
          <motion.div
            key={`flash-${height}`}
            className="pointer-events-none absolute inset-0 bg-gradient-to-t from-cyan/10 to-transparent"
            initial={{ opacity: 0.45 }}
            animate={{ opacity: 0 }}
            exit={{ opacity: 0 }}
            transition={{ duration: 0.55 }}
            aria-hidden
          />
        )}
      </AnimatePresence>
    </div>
  );
}
