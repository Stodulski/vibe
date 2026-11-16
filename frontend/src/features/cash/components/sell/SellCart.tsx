import { ShoppingCart } from 'lucide-react';
import { Panel } from '@/shared/components/common/Panel';
import { DetailSheetFooter } from '@/shared/components/common/DetailSheetFooter';
import { cn } from '@/shared/lib/utils';
import { ES_AR } from '@/shared/i18n/es_AR';
import { type CounterPaymentMethod } from '@/shared/lib/paymentMethods';
import { lineHasStockWarning, type CartLine } from '../../lib/cart';
import type { BalanceLine } from '../../lib/balanceCart';
import { SellCartLine } from './SellCartLine';
import { SellCartFooter } from './SellCartFooter';
import { SELL_CART_PANEL_CLASS } from './sellLayout';
import { SellBalanceCartLine } from './SellBalanceCartLine';
import type { Product } from '@/shared/types/api.types';

const t = ES_AR;

/** The cart's two warnings: the 50-line cap, and a checkout refused because a booking changed. */
function SellCartNotices({
  cartLimitReached,
  balanceChangedNotice,
}: {
  cartLimitReached: boolean;
  balanceChangedNotice: boolean;
}) {
  return (
    <>
      {cartLimitReached && (
        <p role="alert" className="text-warning-text mt-2 text-xs">
          {t.cash.cartMaxLinesReached}
        </p>
      )}
      {balanceChangedNotice && (
        <p role="alert" className="text-warning-text mt-2 text-xs">
          {t.cash.balanceChangedNotice}
        </p>
      )}
    </>
  );
}

interface SellCartLinesProps {
  lines: CartLine[];
  products: Product[];
  balanceLines: BalanceLine[];
  lineErrors: Record<string, string>;
  onIncrement: (productId: string) => void;
  onDecrement: (productId: string) => void;
  onRemove: (productId: string) => void;
  onRemoveBalance: (bookingId: string) => void;
}

/** The scrollable rows: product lines first, then booking balances — split out of `SellCart` so that function stays under this repo's `max-lines-per-function` limit. */
function SellCartLines({
  lines,
  products,
  balanceLines,
  lineErrors,
  onIncrement,
  onDecrement,
  onRemove,
  onRemoveBalance,
}: SellCartLinesProps) {
  return (
    // The list runs into the panel's right padding (-mr/pr) so the scrollbar
    // sits in that gutter, clear of the amounts, which stay aligned with the total.
    <div className="scrollbar-accent -mr-4 min-h-0 flex-1 overflow-x-hidden overflow-y-auto pr-4 sm:-mr-5 sm:pr-5">
      {lines.map((line) => {
        const product = products.find((p) => p.id === line.productId);
        return (
          <SellCartLine
            key={line.productId}
            line={line}
            product={product}
            hasStockWarning={lineHasStockWarning(line, product)}
            error={lineErrors[line.productId]}
            onIncrement={onIncrement}
            onDecrement={onDecrement}
            onRemove={onRemove}
          />
        );
      })}
      {balanceLines.map((line) => (
        <SellBalanceCartLine
          key={line.bookingId}
          line={line}
          error={lineErrors[line.bookingId]}
          onRemove={onRemoveBalance}
        />
      ))}
    </div>
  );
}

function SellCartEmpty() {
  return (
    <div className="flex flex-1 flex-col items-center justify-center gap-3 py-8 text-center">
      <ShoppingCart className="text-text-tertiary size-6" aria-hidden="true" />
      <p className="text-text-secondary text-sm">{t.cash.cartEmpty}</p>
    </div>
  );
}

interface SellCartProps {
  lines: CartLine[];
  products: Product[];
  balanceLines: BalanceLine[];
  total: number;
  method: CounterPaymentMethod;
  onMethodChange: (method: CounterPaymentMethod) => void;
  note: string;
  onNoteChange: (note: string) => void;
  /** Keyed by product id or booking id. */
  lineErrors: Record<string, string>;
  onIncrement: (productId: string) => void;
  onDecrement: (productId: string) => void;
  onRemove: (productId: string) => void;
  onRemoveBalance: (bookingId: string) => void;
  onCharge: () => void;
  isCharging: boolean;
  cartLimitReached: boolean;
  /** The last checkout was refused because a booking changed; the day's list is being refreshed. */
  balanceChangedNotice: boolean;
  /**
   * `column` (default) is the desktop panel. `sheet` is the content of the
   * mobile bottom sheet: no panel of its own, only the lines scroll, and the
   * total and "Cobrar" are pinned in a `DetailSheetFooter`.
   */
  placement?: 'column' | 'sheet';
}

/**
 * The cart panel — shared by the desktop second column and the mobile bottom
 * sheet (`odd/tasks/pos-cashbox.md` T5b: same content, different placement).
 * Product lines first, then the booking balances, in one checkout.
 */
/** The cart's three regions as nodes, so each placement only decides where they sit. */
function cartParts(props: Omit<SellCartProps, 'placement'>) {
  const isEmpty = props.lines.length === 0 && props.balanceLines.length === 0;
  return {
    isEmpty,
    footer: (
      <SellCartFooter
        total={props.total}
        method={props.method}
        onMethodChange={props.onMethodChange}
        note={props.note}
        onNoteChange={props.onNoteChange}
        onCharge={props.onCharge}
        isCharging={props.isCharging}
        chargeDisabled={isEmpty}
      />
    ),
    linesList: (
      <SellCartLines
        lines={props.lines}
        products={props.products}
        balanceLines={props.balanceLines}
        lineErrors={props.lineErrors}
        onIncrement={props.onIncrement}
        onDecrement={props.onDecrement}
        onRemove={props.onRemove}
        onRemoveBalance={props.onRemoveBalance}
      />
    ),
    notices: (
      <SellCartNotices cartLimitReached={props.cartLimitReached} balanceChangedNotice={props.balanceChangedNotice} />
    ),
  };
}

export function SellCart({ placement = 'column', ...props }: SellCartProps) {
  const { isEmpty, footer, linesList, notices } = cartParts(props);

  if (placement === 'sheet') {
    return (
      <>
        {/* The sheet's own height is capped, so this region is what shrinks and
            scrolls; the footer below never leaves the screen. */}
        <div className="flex min-h-0 flex-1 flex-col px-4">
          {isEmpty ? <SellCartEmpty /> : linesList}
          {!isEmpty && notices}
        </div>
        {!isEmpty && <DetailSheetFooter fullWidth>{footer}</DetailSheetFooter>}
      </>
    );
  }

  return (
    // From `lg` the cart is a fixed-height column pinned under the app header:
    // the lines scroll inside it and the total and "Cobrar" never leave the screen.
    // The height leaves a margin below it on purpose: a desktop dock that
    // overlaps the browser window still counts as viewport.
    <Panel size="md" className={cn('flex flex-col', SELL_CART_PANEL_CLASS)} data-testid="sell-cart">
      <h2 className="text-text-primary mb-2 text-lg font-semibold max-lg:sr-only">{t.cash.cartTitle}</h2>

      {isEmpty ? (
        <SellCartEmpty />
      ) : (
        <>
          {linesList}
          {notices}
          {footer}
        </>
      )}
    </Panel>
  );
}
