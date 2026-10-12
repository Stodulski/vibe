import { Navigate } from 'react-router-dom';
import { useAuth } from '@/features/auth/hooks/useAuth';
import { LoadingSpinner } from '@/shared/components/common/LoadingSpinner';
import { Logo } from '@/shared/components/common/Logo';
import { hasSessionHint } from '@/shared/lib/sessionHint';

interface GuestRouteProps {
  children: React.ReactNode;
}

export function GuestRoute({ children }: GuestRouteProps) {
  const { user, isLoading } = useAuth();

  // A browser that has signed in before may be holding a session, so the page
  // waits for the check. The brand mark is carried through, not dropped for a
  // bare spinner: it is already on screen (`index.html`'s app shell draws it
  // before any script runs) and, while this wait lasts, it is the largest
  // contentful element. Replacing it with a spinner blanked it, so the largest
  // paint was re-recorded against the page's own copy a full second later.
  //
  // A first-time visitor has no session to find, so the form is not held back
  // for the check: the page they came for renders at once, and the check only
  // decides whether to send them on, from the form, to the dashboard. The hint
  // is what tells the two apart (see `sessionHint.ts`).
  if (isLoading && hasSessionHint()) {
    return (
      <div className="bg-bg-base flex min-h-dvh flex-col items-center justify-center gap-6">
        <Logo size="lg" />
        <LoadingSpinner size="lg" />
      </div>
    );
  }

  if (user) {
    return <Navigate to="/" replace />;
  }

  return children;
}
