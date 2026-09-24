import { pesosToCentavos } from '@/shared/lib/money';
import type { CreateBookingDto } from '../../schemas/booking.schema';
import type { CreateBookingRequest } from '@/shared/types/api.types';

// Plain if/return (not a ternary/`||`) so an empty string intentionally also
// falls through to the fallback — `??` only catches `null`/`undefined` and
// would keep `''`, which is not the desired behavior here.
function blankToUndefined<T extends string>(value: T | undefined): T | undefined {
  if (!value) return undefined;
  return value;
}

function blankToDefault<T extends string>(value: T | undefined, fallback: T): T {
  if (!value) return fallback;
  return value;
}

/**
 * Normalizes the raw form values before submit: blanks fall through to
 * `undefined`/defaults, and the deposit amount and manual price convert
 * pesos -> cents.
 */
export function cleanBookingPayload(data: CreateBookingDto): CreateBookingRequest {
  const { price, notes, payment_method, deposit_amount, client_email, ...rest } = data;

  const cleanNotes = blankToUndefined(notes);
  const cleanPaymentMethod = blankToUndefined(payment_method);
  const cleanDepositAmount =
    data.payment_option === 'deposit' && deposit_amount ? pesosToCentavos(deposit_amount) : undefined;
  // The manual-price field only ever renders (and gets a value) when
  // there's no server-computed estimate for this span — `price` stays
  // undefined here otherwise, whether or not it was ever briefly populated.
  const cleanPrice = typeof price === 'number' ? pesosToCentavos(price) : undefined;

  // `CreateBookingRequest`'s optional fields are absent-or-present, not
  // present-with-`undefined` (source of truth: `src/shared/types`) — spread
  // each one in only when it actually has a value instead of assigning it
  // `undefined` unconditionally.
  return {
    ...rest,
    payment_option: blankToDefault(data.payment_option, 'unpaid'),
    ...(cleanNotes !== undefined ? { notes: cleanNotes } : {}),
    ...(cleanPaymentMethod !== undefined ? { payment_method: cleanPaymentMethod } : {}),
    ...(cleanDepositAmount !== undefined ? { deposit_amount: cleanDepositAmount } : {}),
    ...(cleanPrice !== undefined ? { price: cleanPrice } : {}),
    ...(client_email !== undefined ? { client_email } : {}),
  };
}
