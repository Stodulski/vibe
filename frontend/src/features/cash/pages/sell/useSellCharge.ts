import { useState } from 'react';
import { getFieldErrors, translateServerError } from '@/shared/lib/serverErrors';
import { blankToUndefined } from '@/shared/lib/blankToUndefined';
import { COUNTER_PAYMENT_METHODS, type CounterPaymentMethod } from '@/shared/lib/paymentMethods';
import { useCreateSale } from '../../hooks/useCreateSale';
import type { CartLine } from '../../lib/cart';
import type { useSellCart } from './useSellCart';
import type { SaleCreateResponse } from '@/shared/types/api.types';
import type { SaleResult } from '../../components/sell/SaleConfirmationDialog';

function withoutLineError(errors: Record<string, string>, productId: string): Record<string, string> {
  if (!(productId in errors)) return errors;
  const rest: Record<string, string> = {};
  for (const [id, message] of Object.entries(errors)) {
    if (id !== productId) rest[id] = message;
  }
  return rest;
}

/** Maps a 422's `items[N].product_id`/`items[N].quantity` field errors back onto the cart line at that same index. */
function mapItemFieldErrors(error: unknown, lines: CartLine[]): Record<string, string> {
  const fieldErrors = getFieldErrors(error);
  const nextLineErrors: Record<string, string> = {};
  lines.forEach((line, index) => {
    const raw = fieldErrors[`items[${String(index)}].product_id`] ?? fieldErrors[`items[${String(index)}].quantity`];
    if (raw) nextLineErrors[line.productId] = translateServerError(raw);
  });
  return nextLineErrors;
}

/**
 * Method, note, the charge mutation and its result/line-error state — split
 * out of `useSellPage` so neither function grows past this repo's
 * `max-lines-per-function` limit.
 */
export function useSellCharge(
  complexId: string | null,
  sessionId: string | null,
  cart: ReturnType<typeof useSellCart>,
) {
  const [method, setMethod] = useState<CounterPaymentMethod>(COUNTER_PAYMENT_METHODS[0]);
  const [note, setNote] = useState('');
  const [lineErrors, setLineErrors] = useState<Record<string, string>>({});
  const [mobileCartOpen, setMobileCartOpen] = useState(false);
  const [saleResult, setSaleResult] = useState<SaleResult | null>(null);
  const createSale = useCreateSale(complexId ?? '', sessionId);

  const removeCartLine = (productId: string) => {
    cart.remove(productId);
    setLineErrors((prev) => withoutLineError(prev, productId));
  };

  const charge = () => {
    if (!complexId || cart.lines.length === 0) return;
    setLineErrors({});
    // Snapshotting the lines the payload is built from — not re-reading
    // `cart.lines` in `onSuccess` — is what lets `removeCharged` below
    // subtract only what was actually sent: anything added or bumped after
    // this point (the mutation is in flight; only the charge button is
    // disabled) must survive the success handler.
    const chargedLines = cart.lines;
    createSale.mutate(
      {
        items: chargedLines.map((line) => ({ product_id: line.productId, quantity: line.quantity })),
        method,
        note: blankToUndefined(note),
      },
      {
        onSuccess: (data: SaleCreateResponse) => {
          setSaleResult({ sale: data.sale, stockWarnings: data.stock_warnings });
          cart.removeCharged(chargedLines);
          setNote('');
          setMethod(COUNTER_PAYMENT_METHODS[0]);
          setMobileCartOpen(false);
        },
        onError: (error: unknown) => {
          setLineErrors(mapItemFieldErrors(error, cart.lines));
        },
      },
    );
  };

  return {
    method,
    setMethod,
    note,
    setNote,
    lineErrors,
    removeCartLine,
    mobileCartOpen,
    setMobileCartOpen,
    saleResult,
    clearSaleResult: () => {
      setSaleResult(null);
    },
    charge,
    isCharging: createSale.isPending,
  };
}
