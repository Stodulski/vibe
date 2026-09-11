import { Outlet } from 'react-router-dom';
import { AppHeader } from './AppHeader';
import { ES_AR } from '@/shared/i18n/es_AR';
import { MeshBackdrop } from './MeshBackdrop';

const t = ES_AR;

export function PublicLayout() {
  return (
    <div className="relative flex min-h-dvh flex-col bg-bg-base">
      <MeshBackdrop />

      {/* Skip navigation link */}
      <a href="#main-content" className="skip-link">
        {t.layout.skipToContent}
      </a>

      <AppHeader className="sticky top-0 z-40" />

      {/* Content */}
      <main id="main-content" className="relative mx-auto flex w-full max-w-2xl flex-1 px-4 py-5 sm:p-8">
        <Outlet />
      </main>

      {/* Footer */}
      <footer className="border-t border-border-default py-8 text-center text-xs leading-4 text-text-tertiary">
        {t.layout.poweredBy}
      </footer>
    </div>
  );
}
