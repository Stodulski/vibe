import { Navigate } from 'react-router-dom';
import { useAuth } from '@/features/auth/hooks/useAuth';
import { LoadingSpinner } from '@/shared/components/common/LoadingSpinner';
import { Logo } from '@/shared/components/common/Logo';

interface GuestRouteProps {
  children: React.ReactNode;
}

export function GuestRoute({ children }: GuestRouteProps) {
  const { user, isLoading } = useAuth();

  // The brand mark is carried through, not dropped for a bare spinner. It is
  // already on screen — `index.html`'s app shell draws it before any script
  // runs — and on /login and /register it is the largest contentful element
  // on the page. Replacing it with a spinner for the length of the session
  // round trip blanked it, so the largest paint was re-recorded against the
  // login form's own copy a full second later.
  if (isLoading) {
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
