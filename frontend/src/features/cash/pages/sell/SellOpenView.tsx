import { Sheet, SheetContent, SheetHeader, SheetTitle } from '@/shared/components/ui/sheet';
import { StaleDataNotice } from '@/shared/components/common/StaleDataNotice';
import { useMediaQuery } from '@/shared/hooks/useMediaQuery';
import { ES_AR } from '@/shared/i18n/es_AR';
import { SellProductGrid } from '../../components/sell/SellProductGrid';
import { SellCart } from '../../components/sell/SellCart';
import { SellMobileCartBar } from '../../components/sell/SellMobileCartBar';
import { SalesSection } from '../../components/sell/SalesSection';
import type { useSellPage } from './useSellPage';

const t = ES_AR;

interface SellMobileCartProps {
  lines: ReturnType<typeof useSellPage>['lines'];
  total: number;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  cartPanel: React.ReactNode;
}

/** The bottom bar + sheet pair for mobile — split out of `SellOpenView` so that function stays under this repo's `max-lines-per-function` limit. */
function SellMobileCart({ lines, total, open, onOpenChange, cartPanel }: SellMobileCartProps) {
  return (
    <>
      <SellMobileCartBar
        lines={lines}
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

/** The catalog-refetch and dropped-stale-lines notices — split out of `SellOpenView` so that function stays under this repo's `max-lines-per-function` limit. */
function SellOpenViewNotices({
  isError,
  onRetry,
  droppedStaleNotice,
}: {
  isError: boolean;
  onRetry: () => void;
  droppedStaleNotice: boolean;
}) {
  return (
    <>
      {isError && <StaleDataNotice onRetry={onRetry} />}
      {droppedStaleNotice && (
        <div
          role="status"
          className="border-warning-border bg-warning-bg text-warning-text rounded-xl border px-3.5 py-2.5 text-sm"
        >
          {t.cash.cartDroppedStaleLines}
        </div>
      )}
    </>
  );
}

interface SellOpenViewProps {
  complexId: string;
  sessionId: string;
  state: ReturnType<typeof useSellPage>;
}

/**
 * The screen once the till is open: catalog grid + cart, and this session's
 * own sales below. Only one `SellCart` is ever mounted at a time (desktop's
 * second column, or the mobile sheet) — rendering both simultaneously (one
 * merely `hidden`) would duplicate the panel's own field ids in the DOM.
 */
export function SellOpenView({ complexId, sessionId, state }: SellOpenViewProps) {
  const isDesktop = useMediaQuery('(min-width: 1024px)');

  const cartPanel = (
    <SellCart
      lines={state.lines}
      products={state.products}
      total={state.total}
      method={state.method}
      onMethodChange={state.setMethod}
      note={state.note}
      onNoteChange={state.setNote}
      lineErrors={state.lineErrors}
      onIncrement={state.incrementCartLine}
      onDecrement={state.decrementCartLine}
      onRemove={state.removeCartLine}
      onCharge={state.charge}
      isCharging={state.isCharging}
      cartLimitReached={state.cartLimitNotice}
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
      />

      <div className="grid grid-cols-1 gap-6 lg:grid-cols-[1fr_360px]">
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
        {isDesktop && cartPanel}
      </div>

      <SalesSection complexId={complexId} sessionId={sessionId} />

      {!isDesktop && (
        <SellMobileCart
          lines={state.lines}
          total={state.total}
          open={state.mobileCartOpen}
          onOpenChange={state.setMobileCartOpen}
          cartPanel={cartPanel}
        />
      )}
    </div>
  );
}
