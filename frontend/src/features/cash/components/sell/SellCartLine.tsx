import { Minus, Plus, X, AlertTriangle } from 'lucide-react';
import { Button } from '@/shared/components/ui/button';
import { formatPrice } from '@/shared/lib/utils';
import { ES_AR } from '@/shared/i18n/es_AR';
import { MAX_LINE_QUANTITY, type CartLine } from '../../lib/cart';
import type { Product } from '@/shared/types/api.types';

const t = ES_AR;

interface SellCartLineProps {
  line: CartLine;
  product: Product | undefined;
  hasStockWarning: boolean;
  /** Server 422 for this line (e.g. the product was deactivated mid-cart) — the owner can only remove it. */
  error?: string | undefined;
  onIncrement: (productId: string) => void;
  onDecrement: (productId: string) => void;
  onRemove: (productId: string) => void;
}

interface QuantityStepperProps {
  name: string;
  quantity: number;
  onIncrement: () => void;
  onDecrement: () => void;
}

/** The −/quantity/+ stepper — split out of `SellCartLine` so that function stays under this repo's `max-lines-per-function` limit. */
function QuantityStepper({ name, quantity, onIncrement, onDecrement }: QuantityStepperProps) {
  return (
    <div className="border-border-subtle flex items-center rounded-lg border">
      <Button
        type="button"
        variant="ghost"
        size="icon-sm"
        aria-label={`${t.cash.cartDecreaseAction}: ${name}`}
        onClick={onDecrement}
      >
        <Minus className="size-3.5" aria-hidden="true" />
      </Button>
      <span className="score-text w-8 text-center text-sm font-semibold" data-testid="sell-cart-line-quantity">
        {quantity}
      </span>
      <Button
        type="button"
        variant="ghost"
        size="icon-sm"
        aria-label={`${t.cash.cartIncreaseAction}: ${name}`}
        disabled={quantity >= MAX_LINE_QUANTITY}
        onClick={onIncrement}
      >
        <Plus className="size-3.5" aria-hidden="true" />
      </Button>
    </div>
  );
}

/** One cart row: name, unit price, quantity stepper, remove — and a non-blocking stock warning when it applies. */
export function SellCartLine({
  line,
  product,
  hasStockWarning,
  error,
  onIncrement,
  onDecrement,
  onRemove,
}: SellCartLineProps) {
  const name = product?.name ?? line.productId;
  const unitPrice = product?.price ?? 0;

  return (
    <div className="border-border-subtle border-b py-3 last:border-b-0">
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0 flex-1">
          <p className="text-text-primary truncate text-sm font-medium">{name}</p>
          <p className="text-text-tertiary text-xs">{formatPrice(unitPrice)}</p>
        </div>
        <Button
          type="button"
          variant="ghost"
          size="icon-sm"
          aria-label={`${t.cash.cartRemove}: ${name}`}
          onClick={() => {
            onRemove(line.productId);
          }}
        >
          <X className="size-4" aria-hidden="true" />
        </Button>
      </div>

      <div className="mt-2 flex items-center justify-between gap-3">
        <QuantityStepper
          name={name}
          quantity={line.quantity}
          onIncrement={() => {
            onIncrement(line.productId);
          }}
          onDecrement={() => {
            onDecrement(line.productId);
          }}
        />
        <span className="score-text text-text-primary text-sm font-semibold">
          {formatPrice(unitPrice * line.quantity)}
        </span>
      </div>

      {hasStockWarning && !error && (
        <p className="text-warning-text mt-1.5 flex items-center gap-1 text-xs">
          <AlertTriangle className="size-3.5 shrink-0" aria-hidden="true" />
          {t.cash.cartLineStockWarning}
        </p>
      )}
      {error && (
        <p role="alert" className="text-error-text mt-1.5 flex items-center gap-1 text-xs">
          <AlertTriangle className="size-3.5 shrink-0" aria-hidden="true" />
          {error}
        </p>
      )}
    </div>
  );
}
