import { useParams, Navigate } from 'react-router-dom';
import { HTTPError } from 'ky';
import { AlertCircle } from 'lucide-react';
import { usePageTitle } from '@/shared/hooks/usePageTitle';
import { useAdminUserDetail, UserDetailPanel } from '@/features/admin';
import { EmptyState } from '@/shared/components/common/EmptyState';
import { Skeleton } from '@/shared/components/ui/skeleton';
import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;

function SkeletonDetail() {
  return (
    <div className="space-y-6">
      <Skeleton className="h-4 w-32" />
      <Skeleton className="h-48 rounded-lg" />
      <Skeleton className="h-32 rounded-lg" />
    </div>
  );
}

export default function AdminUserDetailPage() {
  const { id } = useParams<{ id: string }>();
  usePageTitle(t.admin.detail.userInfo);

  const { data, isLoading, isError, error, refetch } = useAdminUserDetail(id ?? '');

  if (!id) return <Navigate to="/admin/users" replace />;
  if (isLoading) return <SkeletonDetail />;

  if (isError) {
    // A 404 means the id never resolved to a user; anything else (500, a
    // dropped connection) is recoverable, so it gets a retry instead of the
    // same dead-end redirect.
    const notFound = error instanceof HTTPError && error.response.status === 404;
    if (notFound) return <Navigate to="/admin/users" replace />;
    return (
      <EmptyState
        icon={AlertCircle}
        title={t.common.error}
        description={t.admin.users.loadError}
        actionLabel={t.common.refresh}
        onAction={() => {
          void refetch();
        }}
      />
    );
  }

  if (!data) return <Navigate to="/admin/users" replace />;

  return <UserDetailPanel user={data.user} complexes={data.complexes} />;
}
