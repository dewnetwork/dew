/** Deterministic monogram avatar from address — no external API. */

function hueFromAddress(addr: string): number {
  let h = 0;
  const a = addr.toLowerCase().replace(/^0x/, "");
  for (let i = 0; i < a.length; i++) {
    h = (h * 31 + a.charCodeAt(i)) >>> 0;
  }
  return h % 360;
}

export function Identicon({ address, size = 40 }: { address: string; size?: number }) {
  const h = hueFromAddress(address);
  const bg = `hsl(${h} 45% 28%)`;
  const fg = `hsl(${(h + 40) % 360} 70% 72%)`;
  const letters = address.replace(/^0x/i, "").slice(0, 2).toUpperCase();
  return (
    <div
      className="surface-xl flex shrink-0 items-center justify-center text-sm font-bold"
      style={{
        width: size,
        height: size,
        background: `linear-gradient(145deg, ${bg}, hsl(${(h + 20) % 360} 40% 18%))`,
        color: fg,
        boxShadow: "inset 0 1px 0 rgb(255 255 255 / 0.08)",
      }}
      aria-hidden
    >
      {letters}
    </div>
  );
}
