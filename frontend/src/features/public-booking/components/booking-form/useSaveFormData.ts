import { useEffect } from 'react';
import type { UseFormWatch } from 'react-hook-form';
import type { PublicBookingFormData } from '../../schemas/public-booking.schema';
import { parsePhoneWithPrefix } from '@/shared/lib/phone';
import { safeLocalStorage } from '@/shared/lib/safeStorage';
import { STORAGE_KEY, type SavedClientData } from './savedFormData';

/**
 * Persists the client-identifying fields (not notes) to localStorage for
 * quick-book pre-fill on a future visit.
 *
 * Subscribes to `watch` rather than taking `watch()`'s return value as a
 * dependency: calling `watch()` builds a new object every render, so an
 * effect keyed on it used to fire — and write to localStorage — on every
 * render, including while this form sits unmounted-in-spirit behind
 * quick-book mode. `watch(callback)` only invokes the callback when a field
 * actually changes.
 */
export function useSaveFormData(watch: UseFormWatch<PublicBookingFormData>) {
  useEffect(() => {
    const subscription = watch((values) => {
      // The prefix is always +54 now — only the local number is worth
      // persisting; `phone_prefix` is no longer written (see savedFormData.ts).
      const parsed = parsePhoneWithPrefix(values.client_phone ?? '');
      const clientData: SavedClientData = {
        client_first_name: values.client_first_name,
        client_last_name: values.client_last_name,
        client_email: values.client_email,
        client_phone: parsed.localNumber,
      };
      safeLocalStorage.set(STORAGE_KEY, JSON.stringify(clientData));
    });
    return () => {
      subscription.unsubscribe();
    };
  }, [watch]);
}
