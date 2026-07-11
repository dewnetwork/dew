/** Shared mark from /favicon.svg — used in header, footer, empty states. */

type BrandLogoProps = {
  size?: number;
  className?: string;
  title?: string;
};

export function BrandLogo({ size = 32, className = "", title = "Dew" }: BrandLogoProps) {
  return (
    <img
      src="/logo.svg"
      width={size}
      height={size}
      alt={title}
      className={`shrink-0 select-none ${className}`}
      draggable={false}
    />
  );
}
