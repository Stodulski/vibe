import { Navigate } from 'react-router-dom';
import { useStore } from '@/shared/stores';

export function RootRedirect() {
  const user = useStore((s) => s.user);
  return <Navigate to={user?.role === 'superadmin' ? '/admin' : '/complexes'} replace />;
}
