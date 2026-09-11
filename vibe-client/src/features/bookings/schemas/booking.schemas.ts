import { z } from 'zod';
import { ES_AR } from '@/shared/i18n/es_AR';
import { phoneField, firstNameField, lastNameField, optionalEmailField } from '@/shared/lib/validations';
import { parseHhMm, parseYmd } from '@/shared/lib/time';
import { nowInArgentina } from '../lib/today';

const t = ES_AR;

export const createBookingSchema = z
  .object({
    court_id: z.string().min(1, t.validation.selectCourt),
    date: z.string().min(1, t.validation.dateRequired),
    // Slot availability is validated server-side (409 on conflict); the UI only shows
    // available slots so client-side validation is not necessary.
    start_time: z.string().min(1, t.validation.timeSlotRequired),
    client_phone: phoneField,
    client_first_name: firstNameField,
    client_last_name: lastNameField,
    client_email: optionalEmailField,
    duration_minutes: z.union([z.literal(60), z.literal(90), z.literal(120)], {
      message: t.validation.durationInvalid,
    }),
    payment_option: z.enum(['unpaid', 'deposit', 'full']).optional(),
    deposit_amount: z.number().positive(t.validation.amountPositive).optional(),
    payment_method: z.enum(['cash', 'transfer']).optional(),
    notes: z.string().max(500, t.validation.maxChars500).optional().or(z.literal('')),
    // Pesos, converted to centavos in `cleanBookingPayload`. Required exactly
    // when no price rule covers the booking's span — see `PriceOrManualPriceField`
    // and `CreateBookingSteps`'s manual `priceRequired` check, which enforce
    // that pairing outside the schema (it depends on `courts`/`schedules`,
    // neither of which is a form field).
    price: z.number().positive(t.validation.amountPositive).optional(),
  })
  .refine(
    ({ payment_option, payment_method }) => {
      if (payment_option === 'deposit' || payment_option === 'full') {
        return !!payment_method;
      }
      return true;
    },
    { message: t.validation.selectPaymentMethod, path: ['payment_method'] },
  )
  .refine(
    ({ date }) => {
      if (!date) return true;
      const now = nowInArgentina();
      const todayStr = `${String(now.getFullYear())}-${String(now.getMonth() + 1).padStart(2, '0')}-${String(now.getDate()).padStart(2, '0')}`;
      return date >= todayStr;
    },
    { message: t.validation.pastDate, path: ['date'] },
  )
  .refine(
    ({ date, start_time }) => {
      if (!date || !start_time) return true;
      // Compare against current Argentina time. We avoid
      // new Date(`${date}T${time}`), which would use the browser's own zone.
      const now = nowInArgentina();
      const ymd = parseYmd(date);
      const hhMm = parseHhMm(start_time);
      // `date`/`start_time` are always well-formed here (upstream fields already
      // enforce "YYYY-MM-DD"/"HH:MM" via the date/time pickers). If parsing ever
      // fails, mirror the prior `new Date(NaN, ...)` behavior: an invalid slot
      // date compares as `false` against `now`, so the refine fails and the
      // form surfaces the time-in-past error.
      if (!ymd || !hhMm) return false;
      const slotInArgentina = new Date(ymd.year, ymd.month - 1, ymd.day, hhMm.hours, hhMm.minutes);
      return slotInArgentina > now;
    },
    { message: t.validation.timeInPast, path: ['start_time'] },
  );

export const confirmPaymentSchema = z.object({
  method: z.enum(['cash', 'transfer'], {
    message: t.validation.selectPaymentMethod,
  }),
  amount: z.number().positive(t.validation.amountPositive),
});

export const cancelBookingSchema = z.object({
  reason: z.string().max(500, t.validation.maxChars500).optional().or(z.literal('')),
});

export type CreateBookingDto = z.infer<typeof createBookingSchema>;
export type ConfirmPaymentDto = z.infer<typeof confirmPaymentSchema>;
export type CancelBookingDto = z.infer<typeof cancelBookingSchema>;
