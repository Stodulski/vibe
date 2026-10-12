import { useState } from 'react';
import { getFieldErrors, isBookingBalanceConflict, translateServerError } from '@/shared/lib/serverErrors';
import { blankToUndefined } from '@/shared/lib/blankToUndefined';
import { COUNTER_PAYMENT_METHODS, type CounterPaymentMethod } from '@/shared/lib/paymentMethods';
import { useCreateSale } from '../../hooks/useCreateSale';
import type { CartLine } from '../../lib/cart';
import type { BalanceLine } from '../../lib/balanceCart';
import type { useSellCart } from './useSellCart';
import type { useSellBalances } from './useSellBalances';
import type { CreateSaleRequest, SaleCreateResponse } from '@/shared/types/api.types';
import type { SaleResult } from '../../components/sell/SaleConfirmationDialog';

function withoutLineError(errors: Record<string, string>, id: string): Record<string, string> {
  if (!(id in errors)) return errors;
  const rest: Record<string, string> = {};
  for (const [key, message] of Object.entries(errors)) {
    if (key !== id) rest[key] = message;
  }
  return rest;
}

/**
 * Maps a 422's per-field errors back onto the lines the request was built
 * from, by index: `items[N]` onto the product line at N, `booking_payments[N]`
 * onto the balance at N. Takes the lines the request was sent with, not the
 * cart as it is now, since the cart may have changed while the request ran.
 */
function mapLineErrors(error: unknown, products: CartLine[], balances: BalanceLine[]): Record<string, string> {
  const fieldErrors = getFieldErrors(error);
  const nextLineErrors: Record<string, string> = {};
  products.forEach((line, index) => {
    const raw = fieldErrors[`items[${String(index)}].product_id`] ?? fieldErrors[`items[${String(index)}].quantity`];
    if (raw) nextLineErrors[line.productId] = translateServerError(raw);
  });
  balances.forEach((line, index) => {
    const raw =
      fieldErrors[`booking_payments[${String(index)}].booking_id`] ??
      fieldErrors[`booking_payments[${String(index)}].amount`];
    if (raw) nextLineErrors[line.bookingId] = translateServerError(raw);
  });
  return nextLineErrors;
}

/** The checkout request: product `items` (possibly none) and the balances as `booking_payments`. */
function toCheckoutRequest(
  products: CartLine[],
  balances: BalanceLine[],
  method: CounterPaymentMethod,
  note: string,
): CreateSaleRequest {
  return {
    items: products.map((line) => ({ product_id: line.productId, quantity: line.quantity })),
    booking_payments: balances.map((line) => ({ booking_id: line.bookingId, amount: line.amount })),
    method,
    note: blankToUndefined(note),
  };
}

function toSaleResult(method: CounterPaymentMethod, data: SaleCreateResponse): SaleResult {
  return {
    method,
    sale: data.sale,
    bookingPayments: data.booking_payments,
    stockWarnings: data.stock_warnings,
  };
}

/**
 * Method, note, the charge mutation and its result/line-error state — split
 * out of `useSellPage` so neither function grows past this repo's
 * `max-lines-per-function` limit.
 *
 * One checkout carries both kinds of line: product `items` and the booking
 * balances (`booking_payments`). A checkout with only balances is valid; the
 * server returns `sale: null` for it.
 */
export function useSellCharge(
  complexId: string | null,
  sessionId: string | null,
  cart: ReturnType<typeof useSellCart>,
  balances: ReturnType<typeof useSellBalances>,
) {
  const [method, setMethod] = useState<CounterPaymentMethod>(COUNTER_PAYMENT_METHODS[0]);
  const [note, setNote] = useState('');
  const [lineErrors, setLineErrors] = useState<Record<string, string>>({});
  const [balanceChangedNotice, setBalanceChangedNotice] = useState(false);
  const [mobileCartOpen, setMobileCartOpen] = useState(false);
  const [saleResult, setSaleResult] = useState<SaleResult | null>(null);
  const createSale = useCreateSale(complexId ?? '', sessionId);

  const removeCartLine = (productId: string) => {
    cart.remove(productId);
    setLineErrors((prev) => withoutLineError(prev, productId));
  };

  const removeBalanceLine = (bookingId: string) => {
    balances.remove(bookingId);
    setLineErrors((prev) => withoutLineError(prev, bookingId));
  };

  const charge = () => {
    if (!complexId || (cart.lines.length === 0 && balances.lines.length === 0)) return;
    setLineErrors({});
    setBalanceChangedNotice(false);
    // Snapshot what the payload is built from: `removeCharged` must subtract
    // only what was sent, so lines added while the request is in flight survive.
    const [chargedLines, chargedBalances] = [cart.lines, balances.lines];
    createSale.mutate(toCheckoutRequest(chargedLines, chargedBalances, method, note), {
      onSuccess: (data: SaleCreateResponse) => {
        setSaleResult(toSaleResult(method, data));
        cart.removeCharged(chargedLines);
        balances.removeCharged(chargedBalances.map((line) => line.bookingId));
        setNote('');
        setMethod(COUNTER_PAYMENT_METHODS[0]);
        setMobileCartOpen(false);
      },
      onError: (error: unknown) => {
        setLineErrors(mapLineErrors(error, chargedLines, chargedBalances));
        // A balance that changed or stopped being payable: `useCreateSale` refetches
        // the day's list, and `useSellBalances` drops the line that no longer owes.
        setBalanceChangedNotice(isBookingBalanceConflict(error));
      },
    });
  };

  return {
    method,
    setMethod,
    note,
    setNote,
    lineErrors,
    removeCartLine,
    removeBalanceLine,
    balanceChangedNotice,
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
