import { useCallback, useState, type ReactNode } from 'react';
import { Outlet } from 'react-router-dom';
import { MobileNavSheet } from './MobileNavSheet';
import { AppMasthead } from './AppMasthead';
import { PageContentWidth } from './PageContentWidth';
import { MeshBackdrop } from './MeshBackdrop';
import { ES_AR } from '@/shared/i18n/es_AR';

interface AppShellProps {
  /** Full-width desktop masthead brand (logo, optionally with a brand label) — sits above the sidebar and content. */
  brand: ReactNode;
  /** Fully-configured desktop sidebar (e.g. `<Sidebar />`). */
  sidebar: ReactNode;
  /** Renders the sidebar for the mobile sheet, given the callback that should close it on navigation. */
  renderMobileSidebar: (onNavigate: () => void) => ReactNode;
  /** Renders the mobile header, given the callback that opens the mobile sheet. */
  header: (onMenuClick: () => void) => ReactNode;
  /** Extra siblings rendered alongside the shell (e.g. CommandPalette, dialogs) — dashboard-only bits live here. */
  children?: ReactNode;
}

/**
 * Owns the outer "authenticated app shell" structure shared by the owner dashboard and admin layouts:
 * skip-link, desktop masthead, desktop sidebar slot, mobile sheet slot, header slot, and the main
 * content wrapper.
 */
export function AppShell({ brand, sidebar, renderMobileSidebar, header, children }: AppShellProps) {
  const [mobileOpen, setMobileOpen] = useState(false);
  const handleCloseMobile = useCallback(() => {
    setMobileOpen(false);
  }, []);
  const handleOpenMobile = useCallback(() => {
    setMobileOpen(true);
  }, []);

  return (
    // The page scrolls, not an inner pane. The shell used to be `h-dvh` with
    // `<main>` as the only scroller, which put the scrollbar inset from the
    // window edge instead of on it. The chrome keeps its place by being
    // `fixed` rather than by sitting inside a box that never scrolls.
    <div className="bg-bg-base relative min-h-dvh">
      <MeshBackdrop />

      {/* Skip navigation link */}
      <a href="#main-content" className="skip-link">
        {ES_AR.layout.skipToContent}
      </a>

      {/* One bar, two variants: the masthead takes over from `md` up and the
          mobile header below it, so exactly one is ever `CHROME_HEIGHT` tall
          and the offset below is the same at every width. */}
      <div className="fixed inset-x-0 top-0 z-30">
        <AppMasthead brand={brand} />
        {header(handleOpenMobile)}
      </div>

      <div className="fixed top-16 bottom-0 left-0 z-20 hidden overflow-y-auto md:flex">{sidebar}</div>

      <MobileNavSheet open={mobileOpen} onOpenChange={setMobileOpen} sidebar={renderMobileSidebar(handleCloseMobile)} />

      {/* Offsets, not layout: the fixed chrome is out of flow, so the content
          reserves its space by hand — `mt-16` for the bar, `md:ml-[68px]` for
          the rail, both matching the sizes those components set themselves. */}
      <main
        id="main-content"
        className="relative z-10 mt-16 px-3 py-4 sm:px-6 sm:py-6 md:ml-[68px] lg:px-10 lg:py-8 xl:px-14"
      >
        <PageContentWidth>
          <Outlet />
        </PageContentWidth>
      </main>

      {children}
    </div>
  );
}
