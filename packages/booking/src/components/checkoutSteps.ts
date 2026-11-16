import { BOOKING_MESSAGES } from '../messages';

const t = BOOKING_MESSAGES;

/** The steps of a booking: the slot, the person's data, the payment. */
export const BOOKING_STEPS = [t.publicBooking.stepSelect, t.publicBooking.stepData, t.publicBooking.stepPay] as const;
