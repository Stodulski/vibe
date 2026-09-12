import type { ReactNode } from 'react';
import { Search } from 'lucide-react';
import { useCurrentPageHeading } from './page-heading/usePageHeading';
import { useLiveClock } from '@/shared/hooks/useLiveClock';
import { PageContentWidth } from './PageContentWidth';
import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;

// A keyboard-shortcut glyph, not translatable copy — same reasoning as an
// icon. Read once: the platform doesn't change over a session.
const SHORTCUT_HINT = typeof navigator !== 'undefined' && /mac/i.test(navigator.userAgent) ? '⌘K' : 'Ctrl K';

interface AppMastheadProps {
  /** Logo (+ optional brand label) — matches the sidebar's permanent icon-only width. */
  brand: ReactNode;
  /** Shows the search/command-palette trigger — only where `<CommandPalette />` is mounted (UI-11). */
  showSearchTrigger?: boolean | undefined;
}

/**
 * Full-width desktop bar: a brand cell sized to match the (permanently
 * collapsed) sidebar below it, plus the current page's title with the live
 * date/time underneath — same subtitle on every page, published via
 * `usePageHeading`. Kept as its own component so `AppShell` stays under the
 * repo's max-lines-per-function cap.
 */
export function AppMasthead({ brand, showSearchTrigger = false }: AppMastheadProps) {
  const heading = useCurrentPageHeading();
  const clock = useLiveClock();

  return (
    <div className="border-border-subtle bg-bg-base hidden h-16 shrink-0 items-stretch border-b md:flex">
      <div className="flex w-[68px] shrink-0 items-center justify-center">{brand}</div>

      <div className="border-border-subtle flex min-w-0 flex-1 items-center justify-between gap-3 border-l px-3 sm:px-6 lg:px-10 xl:px-14">
        {heading ? (
          <PageContentWidth className="flex min-w-0 flex-col justify-center gap-0.5">
            <h1 className="text-text-primary truncate text-sm font-semibold sm:text-base">{heading.title}</h1>
            <p className="text-text-tertiary truncate text-xs first-letter:uppercase">{clock}</p>
          </PageContentWidth>
        ) : (
          <span />
        )}
        {showSearchTrigger && <CommandPaletteButton />}
      </div>
    </div>
  );
}

/**
 * Makes the command palette's Cmd/Ctrl+K shortcut discoverable — it used to
 * exist with no visible entry point anywhere in the UI (UI-11). Dispatches
 * the same `open-command-palette` event `CommandPalette` already listens for.
 */
function CommandPaletteButton() {
  return (
    <button
      type="button"
      onClick={() => {
        window.dispatchEvent(new Event('open-command-palette'));
      }}
      className="border-border-subtle bg-bg-subtle text-text-tertiary hover:bg-bg-highlight hover:text-text-secondary flex shrink-0 items-center gap-2 rounded-lg border px-3 py-1.5 text-xs transition-colors"
    >
      <Search className="size-3.5" aria-hidden="true" />
      <span className="hidden lg:inline">{t.commandPalette.title}</span>
      <kbd className="border-border-subtle bg-bg-base text-text-tertiary rounded border px-1.5 py-0.5 font-mono text-[10px]">
        {SHORTCUT_HINT}
      </kbd>
    </button>
  );
}
