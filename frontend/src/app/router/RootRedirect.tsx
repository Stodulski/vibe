import { Navigate } from 'react-router-dom';
import { useAuth } from '@/features/auth/hooks/useAuth';

export function RootRedirect() {
  const { user } = useAuth();
  return <Navigate to={user?.role === 'superadmin' ? '/admin' : '/dashboard'} replace />;
}
