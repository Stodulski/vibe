import { z } from 'zod';
import { STORAGE_KEYS } from '@/shared/lib/storageKeys';
import { safeLocalStorage } from '@/shared/lib/safeStorage';

// Re-exported under the local name this feature already uses, but derived from
// the shared registry rather than spelled out again. The literal lived in both
// places, which is the collision storageKeys.ts exists to prevent.
export const STORAGE_KEY = STORAGE_KEYS.BOOKING_CLIENT_DATA;

/**
 * localStorage is editable by hand and its format can drift between deploys,
 * so every field is validated as the plain string it needs to be rather than
 * cast in from whatever `JSON.parse` produced — a stored number or `null`
 * used to reach `.trim()` in `hasCompleteSavedData` and the `PhoneInput`
 * untouched. `safeLocalStorage.getJSON` also covers storage being unreadable
 * outright (private browsing, disabled storage).
 */
export const savedClientDataSchema = z.object({
  client_first_name: z.string().optional(),
  client_last_name: z.string().optional(),
  client_phone: z.string().optional(),
  client_email: z.string().optional(),
  client_notes: z.string().optional(),
  // No longer written (the app only ever uses +54 now) and no longer read —
  // kept optional purely so an old record with this field still parses
  // instead of failing schema validation outright.
  phone_prefix: z.string().optional(),
});

export type SavedClientData = z.infer<typeof savedClientDataSchema>;

export function getSavedFormData(): SavedClientData {
  return safeLocalStorage.getJSON(STORAGE_KEY, savedClientDataSchema) ?? {};
}

export function hasCompleteSavedData(data: SavedClientData): boolean {
  return !!(
    data.client_first_name?.trim() &&
    data.client_last_name?.trim() &&
    data.client_phone?.trim() &&
    data.client_email?.trim()
  );
}
