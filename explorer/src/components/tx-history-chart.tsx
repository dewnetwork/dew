import {
  Area,
  AreaChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";
import type { DayTxPoint } from "@/lib/network-stats";
import { formatNumber } from "@/lib/format";

/** Compact history chart — uses theme CSS vars (matches explorer light/dark). */
export function TxHistoryChart({
  data,
  variant = "panel",
}: {
  data: DayTxPoint[];
  variant?: "panel" | "default";
}) {
  if (data.length === 0) {
    return (
      <div className="flex h-full min-h-[7rem] items-center justify-center text-xs text-muted">
        No history yet
      </div>
    );
  }

  const compact = variant === "panel";

  return (
    <div className={compact ? "h-[7.5rem] w-full sm:h-[8.5rem]" : "h-56 w-full sm:h-64"}>
      <ResponsiveContainer width="100%" height="100%">
        <AreaChart data={data} margin={{ top: 4, right: 4, left: 0, bottom: 0 }}>
          <defs>
            <linearGradient id="txFillTheme" x1="0" y1="0" x2="0" y2="1">
              <stop offset="0%" stopColor="var(--ex-link)" stopOpacity={0.28} />
              <stop offset="100%" stopColor="var(--ex-link)" stopOpacity={0.02} />
            </linearGradient>
          </defs>
          <XAxis
            dataKey="label"
            tick={{ fill: "var(--ex-muted)", fontSize: 10 }}
            tickLine={false}
            axisLine={false}
            interval="preserveStartEnd"
            minTickGap={28}
          />
          <YAxis
            tick={{ fill: "var(--ex-muted)", fontSize: 10 }}
            tickLine={false}
            axisLine={false}
            width={40}
            tickFormatter={(v: number) =>
              v >= 1_000_000
                ? `${(v / 1_000_000).toFixed(0)}M`
                : v >= 1_000
                  ? `${(v / 1_000).toFixed(0)}k`
                  : String(v)
            }
          />
          <Tooltip
            contentStyle={{
              background: "var(--ex-panel)",
              border: "1px solid var(--ex-line)",
              borderRadius: "0.75rem",
              color: "var(--ex-text)",
              fontSize: 12,
              boxShadow: "var(--ex-shadow-panel)",
            }}
            labelStyle={{ color: "var(--ex-muted)" }}
            formatter={(value) => [formatNumber(Number(value ?? 0)), "Txns"]}
            labelFormatter={(_, payload) => {
              const p = payload?.[0]?.payload as DayTxPoint | undefined;
              return p ? `${p.dayKey} (UTC)` : "";
            }}
          />
          <Area
            type="monotone"
            dataKey="txs"
            stroke="var(--ex-link)"
            strokeWidth={1.75}
            fill="url(#txFillTheme)"
            isAnimationActive={false}
          />
        </AreaChart>
      </ResponsiveContainer>
    </div>
  );
}
