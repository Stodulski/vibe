import { XIcon } from 'lucide-react';
import { Sheet, SheetContent, SheetHeader, SheetTitle } from '@/shared/components/ui/sheet';
import { StaleDataNotice } from '@/shared/components/common/StaleDataNotice';
import { Button } from '@/shared/components/ui/button';
import { useMediaQuery } from '@/shared/hooks/useMediaQuery';
import { ES_AR } from '@/shared/i18n/es_AR';
import { SellProductGrid } from '../../components/sell/SellProductGrid';
import { SellBookingBalances } from '../../components/sell/SellBookingBalances';
import { SellCart } from '../../components/sell/SellCart';
import { SellMobileCartBar } from '../../components/sell/SellMobileCartBar';
import { SalesSection } from '../../components/sell/SalesSection';
import type { useSellPage } from './useSellPage';

const t = ES_AR;

interface SellMobileCartProps {
  itemCount: number;
  total: number;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  cartPanel: React.ReactNode;
}

/** The bottom bar + sheet pair for mobile — split out of `SellOpenView` so that function stays under this repo's `max-lines-per-function` limit. */
function SellMobileCart({ itemCount, total, open, onOpenChange, cartPanel }: SellMobileCartProps) {
  return (
    <>
      <SellMobileCartBar
        itemCount={itemCount}
        total={total}
        onOpenCart={() => {
          onOpenChange(true);
        }}
      />

      <Sheet open={open} onOpenChange={onOpenChange}>
        <SheetContent side="bottom" className="max-h-[85vh] overflow-y-auto">
          <SheetHeader className="pb-0">
            <SheetTitle>{t.cash.cartTitle}</SheetTitle>
          </SheetHeader>
          <div className="px-4 pb-4">{cartPanel}</div>
        </SheetContent>
      </Sheet>
    </>
  );
}

interface SellOpenViewNoticesProps {
  isError: boolean;
  onRetry: () => void;
  droppedStaleNotice: boolean;
  onDismissDroppedStaleNotice: () => void;
  droppedBalancesNotice: boolean;
  onDismissDroppedBalancesNotice: () => void;
}

/** The catalog-refetch and dropped-stale-lines notices — split out of `SellOpenView` so that function stays under this repo's `max-lines-per-function` limit. */
function SellOpenViewNotices({
  isError,
  onRetry,
  droppedStaleNotice,
  onDismissDroppedStaleNotice,
  droppedBalancesNotice,
  onDismissDroppedBalancesNotice,
}: SellOpenViewNoticesProps) {
  return (
    <>
      {isError && <StaleDataNotice onRetry={onRetry} />}
      {droppedStaleNotice && (
        <div
          role="status"
          className="border-warning-border bg-warning-bg text-warning-text flex items-center justify-between gap-3 rounded-xl border px-3.5 py-2.5 text-sm"
        >
          <span>{t.cash.cartDroppedStaleLines}</span>
          <Button variant="ghost" size="icon-sm" aria-label={t.common.close} onClick={onDismissDroppedStaleNotice}>
            <XIcon className="size-4" />
          </Button>
        </div>
      )}
      {droppedBalancesNotice && (
        <div
          role="status"
          className="border-warning-border bg-warning-bg text-warning-text flex items-center justify-between gap-3 rounded-xl border px-3.5 py-2.5 text-sm"
        >
          <span>{t.cash.cartDroppedBalances}</span>
          <Button variant="ghost" size="icon-sm" aria-label={t.common.close} onClick={onDismissDroppedBalancesNotice}>
            <XIcon className="size-4" />
          </Button>
        </div>
      )}
    </>
  );
}

/** Today's booking balances above the catalog grid: the left column on desktop, the full width on mobile. */
function SellCatalogColumn({ state }: { state: ReturnType<typeof useSellPage> }) {
  const inCartIds = new Set(state.balanceLines.map((line) => line.bookingId));

  return (
    <div className="min-w-0 space-y-6">
      <SellBookingBalances
        bookings={state.balanceDue.bookings}
        inCartIds={inCartIds}
        isLoading={state.balanceDue.isLoading}
        isError={state.balanceDue.isError}
        onAdd={state.addBalance}
        onRetry={state.balanceDue.refetch}
      />
      <SellProductGrid
        products={state.visibleProducts}
        hasAnyProducts={state.hasAnyProducts}
        search={state.search}
        onSearchChange={state.setSearch}
        categories={state.categories}
        category={state.category}
        onCategoryChange={state.setCategory}
        onSelect={state.addProduct}
      />
    </div>
  );
}

interface SellOpenViewProps {
  complexId: string;
  sessionId: string;
  state: ReturnType<typeof useSellPage>;
}

/**
 * The screen once the till is open: today's booking balances and the catalog
 * grid on the left, the cart beside them, and this session's own sales below.
 * Only one `SellCart` is ever mounted at a time (desktop's second column, or
 * the mobile sheet) — rendering both simultaneously (one merely `hidden`)
 * would duplicate the panel's own field ids in the DOM.
 */
export function SellOpenView({ complexId, sessionId, state }: SellOpenViewProps) {
  const isDesktop = useMediaQuery('(min-width: 1024px)');

  const cartPanel = (
    <SellCart
      lines={state.lines}
      products={state.products}
      balanceLines={state.balanceLines}
      total={state.total}
      method={state.method}
      onMethodChange={state.setMethod}
      note={state.note}
      onNoteChange={state.setNote}
      lineErrors={state.lineErrors}
      onIncrement={state.incrementCartLine}
      onDecrement={state.decrementCartLine}
      onRemove={state.removeCartLine}
      onRemoveBalance={state.removeBalance}
      onCharge={state.charge}
      isCharging={state.isCharging}
      cartLimitReached={state.cartLimitNotice}
      balanceChangedNotice={state.balanceChangedNotice}
    />
  );

  return (
    <div className="space-y-6 pb-24 lg:pb-0">
      <SellOpenViewNotices
        isError={state.productsQuery.isError}
        onRetry={() => {
          void state.productsQuery.refetch();
        }}
        droppedStaleNotice={state.droppedStaleNotice}
        onDismissDroppedStaleNotice={state.dismissDroppedStaleNotice}
        droppedBalancesNotice={state.droppedBalancesNotice}
        onDismissDroppedBalancesNotice={state.dismissDroppedBalancesNotice}
      />

      <div className="grid grid-cols-1 gap-6 lg:grid-cols-[1fr_360px]">
        <SellCatalogColumn state={state} />
        {isDesktop && cartPanel}
      </div>

      <SalesSection complexId={complexId} sessionId={sessionId} />

      {!isDesktop && (
        <SellMobileCart
          itemCount={state.itemCount}
          total={state.total}
          open={state.mobileCartOpen}
          onOpenChange={state.setMobileCartOpen}
          cartPanel={cartPanel}
        />
      )}
    </div>
  );
}
