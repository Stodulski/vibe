import { useState, useCallback, useEffect, useRef } from 'react';
import { AlertTriangle } from 'lucide-react';
import { toast } from 'sonner';
import { PageHeader } from '@/shared/components/common/PageHeader';
import { SkeletonDashboard } from '@/shared/components/common/Skeletons';
import { EmptyState } from '@/shared/components/common/EmptyState';
import { usePageTitle } from '@/shared/hooks/usePageTitle';
import { ES_AR } from '@/shared/i18n/es_AR';
import { useSelectedComplex } from '@/features/complex';
import { useDashboardStats, useClientInsights } from '@/features/dashboard';
import { PublicLinkBar } from './dashboard/PublicLinkBar';
import { DashboardContent } from './dashboard/DashboardContent';

const t = ES_AR;

/** "Copied" flips back off after a couple seconds, cleaning up its own timer on unmount or a fresh copy. */
function useCopyPublicUrl(publicUrl: string | null) {
  const [copied, setCopied] = useState(false);
  const copiedTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(
    () => () => {
      if (copiedTimeoutRef.current) clearTimeout(copiedTimeoutRef.current);
    },
    [],
  );

  const handleCopy = useCallback(() => {
    if (!publicUrl) return;
    void navigator.clipboard.writeText(publicUrl);
    toast.success(t.dashboard.linkCopied);
    setCopied(true);
    if (copiedTimeoutRef.current) clearTimeout(copiedTimeoutRef.current);
    copiedTimeoutRef.current = setTimeout(() => {
      setCopied(false);
    }, 2000);
  }, [publicUrl]);

  return { copied, handleCopy };
}

export default function DashboardPage() {
  usePageTitle(t.dashboard.title);
  const { complex, selectedComplexId } = useSelectedComplex();
  const { data: stats, isLoading, isError, refetch } = useDashboardStats(selectedComplexId);
  const {
    data: clientInsights,
    isLoading: clientsLoading,
    isError: clientsError,
    refetch: refetchClients,
  } = useClientInsights(selectedComplexId);
  const publicUrl = complex?.slug ? `${import.meta.env.VITE_APP_URL ?? window.location.origin}/${complex.slug}` : null;
  const { copied, handleCopy } = useCopyPublicUrl(publicUrl);
  if (!selectedComplexId) return null;

  return (
    <div className="animate-fade-in">
      <PageHeader title={t.dashboard.title} />

      {publicUrl && <PublicLinkBar publicUrl={publicUrl} copied={copied} onCopy={handleCopy} />}

      {isError ? (
        <EmptyState
          icon={AlertTriangle}
          title={t.common.loadError}
          description={t.common.loadErrorDescription}
          actionLabel={t.layout.retry}
          onAction={() => {
            void refetch();
          }}
        />
      ) : isLoading || !stats ? (
        <SkeletonDashboard />
      ) : (
        <DashboardContent
          stats={stats}
          selectedComplexId={selectedComplexId}
          clientInsights={clientInsights}
          clientsLoading={clientsLoading}
          clientsError={clientsError}
          onRetryClients={() => {
            void refetchClients();
          }}
        />
      )}
    </div>
  );
}
