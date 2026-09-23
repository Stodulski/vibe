import { AlertTriangle, Wallet } from 'lucide-react';
import { Link } from 'react-router-dom';
import { PageHeader } from '@/shared/components/common/PageHeader';
import { EmptyState } from '@/shared/components/common/EmptyState';
import { CashSectionTabs } from '@/shared/components/common/CashSectionTabs';
import { SkeletonTable } from '@/shared/components/common/Skeletons';
import { Button } from '@/shared/components/ui/button';
import { usePageTitle } from '@/shared/hooks/usePageTitle';
import { ES_AR } from '@/shared/i18n/es_AR';
import { useSellPage } from './sell/useSellPage';
import { SellOpenView } from './sell/SellOpenView';
import { SaleConfirmationDialog } from '../components/sell/SaleConfirmationDialog';

const t = ES_AR;

export default function CashSellPage() {
  usePageTitle(t.cash.sellTitle);
  const state = useSellPage();

  if (!state.complexId) return null;

  return (
    <div className="animate-fade-in">
      <PageHeader title={t.cash.sellTitle} />
      <CashSectionTabs />
      <SellPageBody complexId={state.complexId} state={state} />
      {/* One level above the closed/open switch below, the same
          "close result unmount" lesson `ClosedSessionResult`/
          `CashCloseDialogSlot` apply — this dialog's own state must not live
          inside a view a charge's own products/session refetch could unmount. */}
      <SaleConfirmationDialog result={state.saleResult} onClose={state.clearSaleResult} />
    </div>
  );
}

function SellPageBody({ complexId, state }: { complexId: string; state: ReturnType<typeof useSellPage> }) {
  const { cashSession } = state;

  if (cashSession.isLoading) return <SkeletonTable rows={4} />;

  if (cashSession.isRealError && !cashSession.data) {
    return (
      <EmptyState
        icon={AlertTriangle}
        title={t.cash.sellLoadError}
        description={t.common.loadErrorDescription}
        actionLabel={t.layout.retry}
        onAction={() => {
          void cashSession.refetch();
        }}
      />
    );
  }

  if (cashSession.isClosed || !cashSession.data) {
    return <SellNeedsOpenTill />;
  }

  return <SellOpenView complexId={complexId} sessionId={cashSession.data.cash_session.id} state={state} />;
}

/**
 * `EmptyState` has no link action (only a button that calls a handler), so
 * this is its same markup with a real `<Link to="/cash">` in its action slot
 * instead — same reasoning `RestockDialog`'s `ClosedTillNotice` already
 * applies for the same "till closed" message.
 */
function SellNeedsOpenTill() {
  return (
    <section className="animate-fade-in flex flex-col items-center justify-center py-20 text-center">
      <Wallet className="text-text-tertiary mb-4 size-8" aria-hidden="true" />
      <h3 className="text-text-primary text-sm font-semibold">{t.cash.sellNeedsOpenTill}</h3>
      <Button asChild className="mt-6 min-h-12 rounded-lg" size="sm">
        <Link to="/cash">{t.cash.sellGoToShift}</Link>
      </Button>
    </section>
  );
}
