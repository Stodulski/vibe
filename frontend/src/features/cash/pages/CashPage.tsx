import { AlertTriangle } from 'lucide-react';
import { PageHeader } from '@/shared/components/common/PageHeader';
import { EmptyState } from '@/shared/components/common/EmptyState';
import { SkeletonTable } from '@/shared/components/common/Skeletons';
import { CashSectionTabs } from '@/shared/components/common/CashSectionTabs';
import { usePageTitle } from '@/shared/hooks/usePageTitle';
import { ES_AR } from '@/shared/i18n/es_AR';
import { useCashPage } from './cash/useCashPage';
import { ClosedCashView } from './cash/ClosedCashView';
import { OpenCashView } from './cash/OpenCashView';
import { CloseCashSessionDialog } from '../components/CloseCashSessionDialog';
import { StaleDataNotice } from '@/shared/components/common/StaleDataNotice';

const t = ES_AR;

export default function CashPage() {
  usePageTitle(t.cash.title);
  const state = useCashPage();

  if (!state.complexId) return null;

  return (
    <div className="animate-fade-in">
      <PageHeader title={t.cash.title} />
      <CashSectionTabs />
      <CashPageBody complexId={state.complexId} state={state} />
    </div>
  );
}

/** Full-screen retry state — only shown when there is nothing cached to fall back on (see `CashPageBody`). */
function CashLoadError({ onRetry }: { onRetry: () => void }) {
  return (
    <EmptyState
      icon={AlertTriangle}
      title={t.cash.loadError}
      description={t.common.loadErrorDescription}
      actionLabel={t.layout.retry}
      onAction={onRetry}
    />
  );
}

/**
 * The close dialog, one level above the `OpenCashView`/`ClosedCashView`
 * switch below — deliberately not inside either branch, so it is never
 * unmounted by that switch (see `useCashPage`'s `closeTarget` doc comment;
 * T3 review: "close result unmount").
 */
function CashCloseDialogSlot({ complexId, state }: { complexId: string; state: ReturnType<typeof useCashPage> }) {
  return (
    <CloseCashSessionDialog
      open={!!state.closeTarget}
      onClose={() => {
        state.setCloseTarget(null);
      }}
      complexId={complexId}
      sessionId={state.closeTarget?.sessionId ?? ''}
      expectedCash={state.closeTarget?.expectedCash ?? 0}
    />
  );
}

function onOpenCloseDialog(state: ReturnType<typeof useCashPage>) {
  return () => {
    const detail = state.detailQuery.data;
    if (!detail) return;
    state.setCloseTarget({ sessionId: detail.cash_session.id, expectedCash: detail.summary.expected_cash });
  };
}

function CashPageBody({ complexId, state }: { complexId: string; state: ReturnType<typeof useCashPage> }) {
  const { sessionQuery } = state;

  if (sessionQuery.isLoading) return <SkeletonTable rows={4} mobile="flat" />;

  // Only the full-screen error when there is nothing cached to fall back on —
  // a background refetch failure (window focus, reconnect) with `data` still
  // around keeps rendering whatever view that data implies, with a
  // non-blocking notice instead (T3 review: "background refetch error wipes
  // the view", same as ComplexLoadError/PR #119 keeping cached complexes).
  if (sessionQuery.isRealError && !sessionQuery.data) {
    return (
      <CashLoadError
        onRetry={() => {
          void sessionQuery.refetch();
        }}
      />
    );
  }

  const staleNotice = sessionQuery.isRealError && (
    <StaleDataNotice
      onRetry={() => {
        void sessionQuery.refetch();
      }}
    />
  );

  if (sessionQuery.isClosed || !sessionQuery.data) {
    return (
      <div className="space-y-4">
        {staleNotice}
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
        <CashCloseDialogSlot complexId={complexId} state={state} />
      </div>
    );
  }

  return (
    <div className="space-y-4">
      {staleNotice}
      <OpenCashView
        complexId={complexId}
        sessionId={sessionQuery.data.cash_session.id}
        detailQuery={state.detailQuery}
        movementDialog={state.movementDialog}
        onOpenMovementDialog={state.setMovementDialog}
        onCloseMovementDialog={() => {
          state.setMovementDialog(null);
        }}
        onOpenCloseDialog={onOpenCloseDialog(state)}
        voidTarget={state.voidTarget}
        onVoidTarget={state.setVoidTarget}
        onClearVoidTarget={() => {
          state.setVoidTarget(null);
        }}
      />
      <CashCloseDialogSlot complexId={complexId} state={state} />
    </div>
  );
}
