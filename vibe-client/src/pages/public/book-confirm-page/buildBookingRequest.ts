import type { BookingSlotInfo, PublicBookingFormData } from '@/features/public-booking';
import type { DurationMinutes, PublicBookingRequest } from '@/shared/types/api.types';

// `||` (and a `x ? x : y` ternary) would also convert `''` to the fallback,
// but ESLint's PreferNullishOverTernary check flags both as "should use
// `??`" false positives — `??` alone would NOT catch `''`, which IS the
// desired behavior here. An explicit `if` satisfies the rule honestly.
function blankToUndefined(value: string | undefined): string | undefined {
  if (!value) return undefined;
  return value;
}

export function buildBookingRequest(slotInfo: BookingSlotInfo, formData: PublicBookingFormData): PublicBookingRequest {
  const clientNotes = blankToUndefined(formData.client_notes);

  return {
    complex_id: slotInfo.complexId,
    court_id: slotInfo.courtId,
    date: slotInfo.date,
    start_time: slotInfo.startTime,
    duration_minutes: slotInfo.durationMinutes as DurationMinutes,
    client_first_name: formData.client_first_name,
    client_last_name: formData.client_last_name,
    client_phone: formData.client_phone,
    // Optional on the form; the API reads an empty string as no email.
    client_email: formData.client_email ?? '',
    // `client_notes?: string` on `PublicBookingRequest` is absent-or-present,
    // not present-with-`undefined` — only include it when there is one.
    ...(clientNotes !== undefined ? { client_notes: clientNotes } : {}),
  };
}
