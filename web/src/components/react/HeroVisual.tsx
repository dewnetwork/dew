import { useReducedMotion } from 'motion/react';
import { useCallback, useRef, type MouseEvent } from 'react';
import { CommitPulse } from './CommitPulse';

/** Subtle perspective tilt on the hero instrument — no particle noise. */
export function HeroVisual() {
  const reduce = useReducedMotion();
  const ref = useRef<HTMLDivElement>(null);

  const onMove = useCallback(
    (e: MouseEvent<HTMLDivElement>) => {
      const el = ref.current;
      if (!el || reduce) return;

      const rect = el.getBoundingClientRect();
      const px = (e.clientX - rect.left) / rect.width - 0.5;
      const py = (e.clientY - rect.top) / rect.height - 0.5;
      const rotY = px * 7;
      const rotX = -py * 6;

      el.style.transform = `perspective(1000px) rotateX(${rotX}deg) rotateY(${rotY}deg) translateZ(0)`;
    },
    [reduce],
  );

  const onLeave = useCallback(() => {
    const el = ref.current;
    if (!el) return;
    el.style.transform = 'perspective(1000px) rotateX(0deg) rotateY(0deg)';
  }, []);

  return (
    <div className="w-full [perspective:1000px] lg:justify-self-end">
      <div
        ref={ref}
        onMouseMove={onMove}
        onMouseLeave={onLeave}
        className="will-change-transform transition-transform duration-300 ease-out"
        style={{ transformStyle: 'preserve-3d' }}
      >
        <CommitPulse />
      </div>
    </div>
  );
}
