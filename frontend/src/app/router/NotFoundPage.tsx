import { Link } from 'react-router-dom';
import { AppHeader } from '@/shared/components/layout/AppHeader';
import { usePageTitle } from '@/shared/hooks/usePageTitle';
import { ES_AR } from '@/shared/i18n/es_AR';
import { MeshBackdrop } from '@/shared/components/layout/MeshBackdrop';

const t = ES_AR;

export function NotFoundPage() {
  usePageTitle(t.layout.notFoundTitle);
  return (
    <div className="relative flex min-h-dvh flex-col bg-bg-base">
      <MeshBackdrop />
      <AppHeader />
      <div className="flex flex-1 items-center justify-center p-6">
        <div className="text-center">
          <h1 className="text-6xl font-bold text-text-tertiary">404</h1>
          <p className="mt-2 text-lg font-semibold text-text-primary">{t.layout.notFoundTitle}</p>
          <p className="mt-1 text-sm text-text-secondary">{t.layout.notFoundDescription}</p>
          <Link to="/" className="mt-4 inline-block text-sm font-medium text-primary-400 hover:underline">
            {t.layout.backHome}
          </Link>
        </div>
      </div>
    </div>
  );
}
