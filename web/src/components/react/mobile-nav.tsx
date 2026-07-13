import {
  IconBook2,
  IconBrandGithub,
  IconGitBranch,
  IconLayersLinked,
  IconMenu2,
  IconNetwork,
  IconRocket,
  IconTool,
  IconX,
} from '@tabler/icons-react';
import { useCallback, useEffect, useId, useState, type ComponentType } from 'react';

type NavLink = {
  href: string;
  label: string;
  icon: 'network' | 'protocol' | 'builders' | 'phases' | 'docs';
};

const ICONS: Record<
  NavLink['icon'],
  ComponentType<{ size?: number; stroke?: number; className?: string }>
> = {
  network: IconNetwork,
  protocol: IconLayersLinked,
  builders: IconTool,
  phases: IconGitBranch,
  docs: IconBook2,
};

type Props = {
  links: NavLink[];
  tryPublicHref: string;
  githubHref?: string;
};

export function MobileNav({
  links,
  tryPublicHref,
  githubHref = 'https://github.com/dewnetwork/dew',
}: Props) {
  const [open, setOpen] = useState(false);
  const titleId = useId();
  const panelId = useId();

  const close = useCallback(() => setOpen(false), []);
  const toggle = useCallback(() => setOpen((v) => !v), []);

  useEffect(() => {
    if (!open) return;
    const prev = document.body.style.overflow;
    document.body.style.overflow = 'hidden';
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') close();
    };
    window.addEventListener('keydown', onKey);
    return () => {
      document.body.style.overflow = prev;
      window.removeEventListener('keydown', onKey);
    };
  }, [open, close]);

  return (
    <div className="lg:hidden">
      <button
        type="button"
        className="icon-well-sm text-slate"
        aria-expanded={open}
        aria-controls={panelId}
        aria-label={open ? 'Close menu' : 'Open menu'}
        onClick={toggle}
      >
        {open ? (
          <IconX size={18} stroke={1.75} />
        ) : (
          <IconMenu2 size={18} stroke={1.75} />
        )}
      </button>

      {open && (
        <div className="fixed inset-0 z-50 lg:hidden" role="presentation">
          <button
            type="button"
            className="absolute inset-0 bg-ink/70 backdrop-blur-sm"
            aria-label="Close menu"
            onClick={close}
          />
          <div
            id={panelId}
            role="dialog"
            aria-modal="true"
            aria-labelledby={titleId}
            className="mobile-sheet absolute inset-x-0 top-0 border-b border-[var(--color-line)] bg-ink-deep/98 shadow-[var(--shadow-panel)] backdrop-blur-xl"
          >
            <div className="mx-auto flex h-16 max-w-6xl items-center justify-between px-5 sm:px-8">
              <p
                id={titleId}
                className="font-display text-sm font-bold tracking-tight text-frost"
              >
                Menu
              </p>
              <button
                type="button"
                className="icon-well-sm text-slate"
                aria-label="Close menu"
                onClick={close}
              >
                <IconX size={18} stroke={1.75} />
              </button>
            </div>

            <nav
              className="mx-auto max-w-6xl px-5 pb-6 sm:px-8"
              aria-label="Mobile"
            >
              <ul className="m-0 flex list-none flex-col gap-1 p-0">
                {links.map((link) => {
                  const Icon = ICONS[link.icon];
                  return (
                    <li key={link.href} className="m-0 list-none p-0">
                      <a
                        href={link.href}
                        className="flex items-center gap-3 rounded-xl border border-transparent px-3 py-3 text-sm text-slate transition hover:border-[var(--color-line)] hover:bg-panel/60 hover:text-frost"
                        onClick={close}
                      >
                        <span className="icon-well-sm">
                          <Icon size={16} stroke={1.75} />
                        </span>
                        {link.label}
                      </a>
                    </li>
                  );
                })}
              </ul>

              <div className="mt-4 flex flex-col gap-2 border-t border-[var(--color-line)] pt-4">
                <a
                  href={tryPublicHref}
                  className="btn-primary w-full !justify-center px-4 py-3"
                  onClick={close}
                >
                  <IconRocket size={16} stroke={1.75} />
                  Try public testnet
                </a>
                <a
                  href={githubHref}
                  className="btn-secondary w-full !justify-center px-4 py-3"
                  target="_blank"
                  rel="noopener noreferrer"
                  onClick={close}
                >
                  <IconBrandGithub size={16} stroke={1.75} />
                  GitHub
                </a>
              </div>
            </nav>
          </div>
        </div>
      )}
    </div>
  );
}
