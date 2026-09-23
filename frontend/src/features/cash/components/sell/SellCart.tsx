import { Loader2, ShoppingCart } from 'lucide-react';
import { Panel } from '@/shared/components/common/Panel';
import { EmptyState } from '@/shared/components/common/EmptyState';
import { Label } from '@/shared/components/ui/label';
import { Textarea } from '@/shared/components/ui/textarea';
import { Button } from '@/shared/components/ui/button';
import { formatPrice } from '@/shared/lib/utils';
import { ES_AR } from '@/shared/i18n/es_AR';
import type { CounterPaymentMethod } from '@/shared/lib/paymentMethods';
import { lineHasStockWarning, type CartLine } from '../../lib/cart';
import { SellCartLine } from './SellCartLine';
import { MovementMethodField } from '../MovementMethodField';
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

/** Total, method, note and the "Cobrar $X" button — split out of `SellCart` so that function stays under this repo's `max-lines-per-function` limit. */
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
    <div className="border-border-subtle mt-3 space-y-3 border-t pt-3">
      <div className="flex items-center justify-between">
        <span className="text-text-secondary text-sm font-medium">{t.cash.cartTotal}</span>
        <span className="score-text text-text-primary text-lg font-bold" data-testid="sell-cart-total">
          {formatPrice(total)}
        </span>
      </div>

      <MovementMethodField value={method} onChange={onMethodChange} />

      <div className="space-y-1.5">
        <Label htmlFor="sell-cart-note">{`${t.cash.cartNote} (${t.common.optional})`}</Label>
        <Textarea
          id="sell-cart-note"
          value={note}
          onChange={(e) => {
            onNoteChange(e.target.value);
          }}
          maxLength={500}
          className="min-h-[40px] resize-none"
        />
      </div>

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

interface SellCartProps {
  lines: CartLine[];
  products: Product[];
  total: number;
  method: CounterPaymentMethod;
  onMethodChange: (method: CounterPaymentMethod) => void;
  note: string;
  onNoteChange: (note: string) => void;
  lineErrors: Record<string, string>;
  onIncrement: (productId: string) => void;
  onDecrement: (productId: string) => void;
  onRemove: (productId: string) => void;
  onCharge: () => void;
  isCharging: boolean;
  cartLimitReached: boolean;
}

/**
 * The cart panel — shared by the desktop second column and the mobile bottom
 * sheet (`odd/tasks/pos-cashbox.md` T5b: same content, different placement).
 */
export function SellCart({
  lines,
  products,
  total,
  method,
  onMethodChange,
  note,
  onNoteChange,
  lineErrors,
  onIncrement,
  onDecrement,
  onRemove,
  onCharge,
  isCharging,
  cartLimitReached,
}: SellCartProps) {
  return (
    <Panel size="sm" className="flex h-full flex-col" data-testid="sell-cart">
      <h2 className="text-text-tertiary mb-2 text-sm font-semibold tracking-wider uppercase">{t.cash.cartTitle}</h2>

      {lines.length === 0 ? (
        <EmptyState icon={ShoppingCart} title={t.cash.cartEmpty} description="" />
      ) : (
        <>
          <div className="min-h-0 flex-1 overflow-y-auto">
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
          </div>

          {cartLimitReached && (
            <p role="alert" className="text-warning-text mt-2 text-xs">
              {t.cash.cartMaxLinesReached}
            </p>
          )}

          <SellCartFooter
            total={total}
            method={method}
            onMethodChange={onMethodChange}
            note={note}
            onNoteChange={onNoteChange}
            onCharge={onCharge}
            isCharging={isCharging}
            chargeDisabled={lines.length === 0}
          />
        </>
      )}
    </Panel>
  );
}
