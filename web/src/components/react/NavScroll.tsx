import { useEffect } from 'react';

/** Toggles `nav-scrolled` on <html> for sticky nav elevation. */
export function NavScroll() {
  useEffect(() => {
    const root = document.documentElement;
    const onScroll = () => {
      root.classList.toggle('nav-scrolled', window.scrollY > 10);
    };
    onScroll();
    window.addEventListener('scroll', onScroll, { passive: true });
    return () => window.removeEventListener('scroll', onScroll);
  }, []);

  return null;
}
