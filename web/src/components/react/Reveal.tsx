import { motion, useInView, useReducedMotion } from 'motion/react';
import { useEffect, useRef, useState, type ReactNode } from 'react';

type RevealProps = {
  children: ReactNode;
  className?: string;
  delay?: number;
  y?: number;
};

/**
 * Scroll reveal with Astro-safe behavior:
 * - SSR always paints fully visible (no opacity:0 in HTML)
 * - After mount, elements well below the fold arm a hidden state instantly
 * - When they enter the viewport, they fade/slide in once
 */
export function Reveal({
  children,
  className = '',
  delay = 0,
  y = 28,
}: RevealProps) {
  const reduce = useReducedMotion();
  const ref = useRef<HTMLDivElement>(null);
  const inView = useInView(ref, {
    once: true,
    amount: 0.18,
    margin: '0px 0px -8% 0px',
  });

  // ssr  → visible, no motion (matches server HTML)
  // armed → waiting off-screen (instant hide, no transition)
  // live  → visible (animated in from armed)
  const [phase, setPhase] = useState<'ssr' | 'armed' | 'live'>('ssr');

  useEffect(() => {
    if (reduce) {
      setPhase('live');
      return;
    }

    const el = ref.current;
    if (!el) {
      setPhase('live');
      return;
    }

    const rect = el.getBoundingClientRect();
    const vh = window.innerHeight || 0;
    // Only arm when clearly below the fold — keeps hero / in-view blocks stable.
    const wellBelow = rect.top > vh * 0.9;

    setPhase(wellBelow ? 'armed' : 'live');
  }, [reduce]);

  useEffect(() => {
    if (phase === 'armed' && inView) {
      setPhase('live');
    }
  }, [inView, phase]);

  const hidden = phase === 'armed';

  return (
    <motion.div
      ref={ref}
      className={className}
      initial={false}
      animate={hidden ? { opacity: 0, y } : { opacity: 1, y: 0 }}
      transition={
        // Animate only the reveal (armed → live). Arming stays instant.
        phase === 'live' && !reduce
          ? {
              duration: 0.65,
              delay,
              ease: [0.22, 1, 0.36, 1],
            }
          : { duration: 0 }
      }
    >
      {children}
    </motion.div>
  );
}
