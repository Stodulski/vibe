import { ShoppingCart } from 'lucide-react';
import { PageHeader } from '@/shared/components/common/PageHeader';
import { EmptyState } from '@/shared/components/common/EmptyState';
import { CashSectionTabs } from '@/shared/components/common/CashSectionTabs';
import { usePageTitle } from '@/shared/hooks/usePageTitle';
import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;

/**
 * Placeholder for the "Vender" tab (`odd/tasks/pos-cashbox.md` T5a: "a
 * simple placeholder 'Próximamente' for now — T5b fills it; keep it minimal
 * and tested"). T5b replaces this body with product tiles, cart and charge —
 * the route and the tab stay the same.
 */
export default function CashSellPage() {
  usePageTitle(t.cash.sellComingSoonTitle);

  return (
    <div className="animate-fade-in">
      <PageHeader title={t.cash.sellComingSoonTitle} />
      <CashSectionTabs />
      <EmptyState
        icon={ShoppingCart}
        title={t.cash.sellComingSoonTitle}
        description={t.cash.sellComingSoonDescription}
      />
    </div>
  );
}
