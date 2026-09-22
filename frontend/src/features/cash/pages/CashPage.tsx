import { AlertTriangle } from 'lucide-react';
import { PageHeader } from '@/shared/components/common/PageHeader';
import { EmptyState } from '@/shared/components/common/EmptyState';
import { SkeletonTable } from '@/shared/components/common/Skeletons';
import { usePageTitle } from '@/shared/hooks/usePageTitle';
import { ES_AR } from '@/shared/i18n/es_AR';
import { useCashPage } from './cash/useCashPage';
import { ClosedCashView } from './cash/ClosedCashView';
import { OpenCashView } from './cash/OpenCashView';

const t = ES_AR;

export default function CashPage() {
  usePageTitle(t.cash.title);
  const state = useCashPage();

  if (!state.complexId) return null;

  return (
    <div className="animate-fade-in">
      <PageHeader title={t.cash.title} />
      <CashPageBody complexId={state.complexId} state={state} />
    </div>
  );
}

function CashPageBody({ complexId, state }: { complexId: string; state: ReturnType<typeof useCashPage> }) {
  const { sessionQuery } = state;

  if (sessionQuery.isLoading) return <SkeletonTable rows={4} />;

  if (sessionQuery.isRealError) {
    return (
      <EmptyState
        icon={AlertTriangle}
        title={t.cash.loadError}
        description={t.common.loadErrorDescription}
        actionLabel={t.layout.retry}
        onAction={() => {
          void sessionQuery.refetch();
        }}
      />
    );
  }

  if (sessionQuery.isClosed || !sessionQuery.data) {
    return (
      <ClosedCashView
        complexId={complexId}
        openDialogOpen={state.openDialogOpen}
        onOpenDialog={() => {
          state.setOpenDialogOpen(true);
        }}
        onCloseDialog={() => {
          state.setOpenDialogOpen(false);
        }}
      />
    );
  }

  return (
    <OpenCashView
      complexId={complexId}
      sessionId={sessionQuery.data.cash_session.id}
      detailQuery={state.detailQuery}
      movementDialog={state.movementDialog}
      onOpenMovementDialog={state.setMovementDialog}
      onCloseMovementDialog={() => {
        state.setMovementDialog(null);
      }}
      closeDialogOpen={state.closeDialogOpen}
      onOpenCloseDialog={() => {
        state.setCloseDialogOpen(true);
      }}
      onCloseCloseDialog={() => {
        state.setCloseDialogOpen(false);
      }}
      voidTarget={state.voidTarget}
      onVoidTarget={state.setVoidTarget}
      onClearVoidTarget={() => {
        state.setVoidTarget(null);
      }}
    />
  );
}
