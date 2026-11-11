import { useState } from 'react';
import { Loader2, ShoppingCart, StickyNote } from 'lucide-react';
import { Panel } from '@/shared/components/common/Panel';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/shared/components/ui/select';
import { Textarea } from '@/shared/components/ui/textarea';
import { Button } from '@/shared/components/ui/button';
import { formatPrice } from '@/shared/lib/utils';
import { ES_AR } from '@/shared/i18n/es_AR';
import { COUNTER_PAYMENT_METHODS, type CounterPaymentMethod } from '@/shared/lib/paymentMethods';
import { lineHasStockWarning, type CartLine } from '../../lib/cart';
import type { BalanceLine } from '../../lib/balanceCart';
import { SellCartLine } from './SellCartLine';
import { SellBalanceCartLine } from './SellBalanceCartLine';
import type { Product } from '@/shared/types/api.types';

const t = ES_AR;

interface SellCartFooterProps {
  total: number;
  method: CounterPaymentMethod;
  onMethodChange: (method: CounterPaymentMethod) => void;
  note: string;
  onNoteChange: (note: string) => void;
  onCharge: () => void;
  isCharging: boolean;
  chargeDisabled: boolean;
}

/** The payment method, labelled for screen readers; the row has no room for a visible label. */
function CartMethodSelect({
  method,
  onMethodChange,
}: {
  method: CounterPaymentMethod;
  onMethodChange: (method: CounterPaymentMethod) => void;
}) {
  return (
    <Select
      value={method}
      onValueChange={(v) => {
        onMethodChange(v as CounterPaymentMethod);
      }}
    >
      <SelectTrigger aria-label={t.cash.movementMethod} className="h-11 min-w-0 flex-1">
        <SelectValue />
      </SelectTrigger>
      <SelectContent>
        {COUNTER_PAYMENT_METHODS.map((m) => (
          <SelectItem key={m} value={m}>
            {t.bookings.paymentMethods[m]}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  );
}

/**
 * Method and note on one compact row: the method as a full-width select, the
 * note as an icon toggle beside it, since most sales carry none. The open note
 * field shows below the row.
 */
function CartMethodAndNote({
  method,
  onMethodChange,
  note,
  onNoteChange,
}: {
  method: CounterPaymentMethod;
  onMethodChange: (method: CounterPaymentMethod) => void;
  note: string;
  onNoteChange: (note: string) => void;
}) {
  const [noteOpen, setNoteOpen] = useState(note !== '');

  return (
    <>
      <div className="flex items-center gap-2">
        <CartMethodSelect method={method} onMethodChange={onMethodChange} />
        <Button
          type="button"
          variant="outline"
          size="icon-lg"
          className="size-11 shrink-0"
          aria-label={t.cash.cartAddNote}
          aria-pressed={noteOpen}
          onClick={() => {
            setNoteOpen((open) => !open);
          }}
        >
          <StickyNote className="size-4" aria-hidden="true" />
        </Button>
      </div>
      {noteOpen && (
        <Textarea
          id="sell-cart-note"
          aria-label={`${t.cash.cartNote} (${t.common.optional})`}
          placeholder={`${t.cash.cartNote} (${t.common.optional})`}
          value={note}
          onChange={(e) => {
            onNoteChange(e.target.value);
          }}
          maxLength={500}
          className="min-h-[40px] resize-none"
        />
      )}
    </>
  );
}

/**
 * Total, method, note and "Cobrar $X", kept short so the lines above get the
 * room. The charge button is the one green element of the cart.
 */
function SellCartFooter({
  total,
  method,
  onMethodChange,
  note,
  onNoteChange,
  onCharge,
  isCharging,
  chargeDisabled,
}: SellCartFooterProps) {
  return (
    <div className="border-border-subtle mt-2 shrink-0 space-y-3 border-t pt-4">
      <div className="flex items-baseline justify-between gap-3">
        <span className="text-text-secondary text-sm">{t.cash.cartTotal}</span>
        <span
          className="font-display text-text-primary text-2xl font-bold tracking-tight tabular-nums"
          data-testid="sell-cart-total"
        >
          {formatPrice(total)}
        </span>
      </div>

      <CartMethodAndNote method={method} onMethodChange={onMethodChange} note={note} onNoteChange={onNoteChange} />

      <Button
        type="button"
        className="h-12 w-full text-base"
        disabled={isCharging || chargeDisabled}
        onClick={onCharge}
      >
        {isCharging && <Loader2 className="size-4 animate-spin" aria-hidden="true" />}
        {`${t.cash.cartChargeActionPrefix} ${formatPrice(total)}`}
      </Button>
    </div>
  );
}

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
}

/**
 * The cart panel — shared by the desktop second column and the mobile bottom
 * sheet (`odd/tasks/pos-cashbox.md` T5b: same content, different placement).
 * Product lines first, then the booking balances, in one checkout.
 */
export function SellCart({
  lines,
  products,
  balanceLines,
  total,
  method,
  onMethodChange,
  note,
  onNoteChange,
  lineErrors,
  onIncrement,
  onDecrement,
  onRemove,
  onRemoveBalance,
  onCharge,
  isCharging,
  cartLimitReached,
  balanceChangedNotice,
}: SellCartProps) {
  const isEmpty = lines.length === 0 && balanceLines.length === 0;

  return (
    // From `lg` the cart is a fixed-height column pinned under the app header:
    // the lines scroll inside it and the total and "Cobrar" never leave the screen.
    // The height leaves a margin below it on purpose: a desktop dock that
    // overlaps the browser window still counts as viewport.
    <Panel
      size="md"
      className="flex flex-col lg:sticky lg:top-20 lg:h-[calc(100dvh-15rem)] lg:min-h-[24rem]"
      data-testid="sell-cart"
    >
      <h2 className="text-text-primary mb-2 text-lg font-semibold max-lg:sr-only">{t.cash.cartTitle}</h2>

      {isEmpty ? (
        <div className="flex flex-1 flex-col items-center justify-center gap-3 py-8 text-center">
          <ShoppingCart className="text-text-tertiary size-6" aria-hidden="true" />
          <p className="text-text-secondary text-sm">{t.cash.cartEmpty}</p>
        </div>
      ) : (
        <>
          <SellCartLines
            lines={lines}
            products={products}
            balanceLines={balanceLines}
            lineErrors={lineErrors}
            onIncrement={onIncrement}
            onDecrement={onDecrement}
            onRemove={onRemove}
            onRemoveBalance={onRemoveBalance}
          />

          <SellCartNotices cartLimitReached={cartLimitReached} balanceChangedNotice={balanceChangedNotice} />

          <SellCartFooter
            total={total}
            method={method}
            onMethodChange={onMethodChange}
            note={note}
            onNoteChange={onNoteChange}
            onCharge={onCharge}
            isCharging={isCharging}
            chargeDisabled={isEmpty}
          />
        </>
      )}
    </Panel>
  );
}
