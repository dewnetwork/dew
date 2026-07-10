import {
  useCallback,
  useRef,
  type CSSProperties,
  type MouseEvent,
  type ReactNode,
} from 'react';

type GlowCardProps = {
  children: ReactNode;
  className?: string;
};

/**
 * Pointer-tracked dew glow on cards. Reduced-motion users get CSS hover only.
 */
export function GlowCard({ children, className = '' }: GlowCardProps) {
  const ref = useRef<HTMLDivElement>(null);

  const onMove = useCallback((e: MouseEvent<HTMLDivElement>) => {
    const el = ref.current;
    if (!el) return;
    if (window.matchMedia('(prefers-reduced-motion: reduce)').matches) return;

    const rect = el.getBoundingClientRect();
    el.style.setProperty('--glow-x', `${e.clientX - rect.left}px`);
    el.style.setProperty('--glow-y', `${e.clientY - rect.top}px`);
  }, []);

  const onEnter = useCallback(() => {
    ref.current?.classList.add('is-glowing');
  }, []);

  const onLeave = useCallback(() => {
    const el = ref.current;
    if (!el) return;
    el.classList.remove('is-glowing');
    el.style.setProperty('--glow-x', '50%');
    el.style.setProperty('--glow-y', '50%');
  }, []);

  const style = {
    '--glow-x': '50%',
    '--glow-y': '50%',
  } as CSSProperties;

  return (
    <div
      ref={ref}
      className={['card-surface', className].filter(Boolean).join(' ')}
      style={style}
      onMouseMove={onMove}
      onMouseEnter={onEnter}
      onMouseLeave={onLeave}
    >
      {children}
    </div>
  );
}
