import { Link } from 'react-router-dom';
import { AppHeader } from '@/shared/components/layout/AppHeader';
import { usePageTitle } from '@/shared/hooks/usePageTitle';
import { ES_AR } from '@/shared/i18n/es_AR';
import { MeshBackdrop } from '@/shared/components/layout/MeshBackdrop';

const t = ES_AR;

/**
 * Shown by `ProtectedRoute` in place of its children when the signed-in
 * user's role isn't in the route's `allowedRoles` — a real distinction from
 * `NotFoundPage`: the route exists, the person is just not allowed to see
 * it. `ProtectedRoute` used to send them to `/` instead, silently — a valid
 * click that looked like it did nothing.
 */
export function ForbiddenPage() {
  usePageTitle(t.layout.forbiddenTitle);
  return (
    <div className="bg-bg-base relative flex min-h-dvh flex-col">
      <MeshBackdrop />
      <AppHeader />
      <div className="flex flex-1 items-center justify-center p-6">
        <div className="text-center">
          <h1 className="text-text-tertiary text-6xl font-bold">403</h1>
          <p className="text-text-primary mt-2 text-lg font-semibold">{t.layout.forbiddenTitle}</p>
          <p className="text-text-secondary mt-1 text-sm">{t.layout.forbiddenDescription}</p>
          <Link to="/" className="text-primary-400 mt-4 inline-block text-sm font-medium hover:underline">
            {t.layout.backHome}
          </Link>
        </div>
      </div>
    </div>
  );
}
