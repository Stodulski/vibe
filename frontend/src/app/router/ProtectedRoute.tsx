import { Navigate, useLocation } from 'react-router-dom';
import { useAuth } from '@/features/auth/hooks/useAuth';
import type { UserRole } from '@/shared/types/api.types';
import { LoadingSpinner } from '@/shared/components/common/LoadingSpinner';
import { ForbiddenPage } from './ForbiddenPage';

interface ProtectedRouteProps {
  children: React.ReactNode;
  allowedRoles?: UserRole[] | undefined;
}

export function ProtectedRoute({ children, allowedRoles }: ProtectedRouteProps) {
  const { user, isLoading } = useAuth();
  const location = useLocation();

  if (isLoading) {
    return (
      <div className="bg-bg-base flex min-h-dvh items-center justify-center">
        <LoadingSpinner size="lg" />
      </div>
    );
  }

  if (!user) {
    return <Navigate to="/login" state={{ from: location }} replace />;
  }

  // A wrong role is not "not found": the route exists and the click did
  // something, it's just not allowed. Rendered in place instead of a silent
  // redirect to "/", which used to look like the click had no effect at all.
  if (allowedRoles && !allowedRoles.includes(user.role)) {
    return <ForbiddenPage />;
  }

  return children;
}
