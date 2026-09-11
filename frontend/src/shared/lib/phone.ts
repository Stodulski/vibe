import { DEFAULT_PHONE_PREFIX } from './constants';

export function parsePhoneWithPrefix(fullPhone: string): {
  prefix: string;
  localNumber: string;
} {
  // `fullPhone` is typed as a required `string`, but react-hook-form's
  // `Controller` can pass an actually-`undefined` `field.value` at runtime
  // before the field has been touched (its declared form type is a wider lie
  // than what `defaultValues` guarantees on first render). Keep the runtime
  // guard even though TS considers it unreachable — removing it throws.
  // eslint-disable-next-line @typescript-eslint/no-unnecessary-condition -- see comment above
  if (fullPhone?.startsWith(DEFAULT_PHONE_PREFIX)) {
    return { prefix: DEFAULT_PHONE_PREFIX, localNumber: fullPhone.slice(DEFAULT_PHONE_PREFIX.length) };
  }

  // Anything else — no prefix, or a stale/foreign prefix from before the
  // country selector was removed — is treated as the whole local number; the
  // app only ever sends/stores `+54` numbers now.
  // eslint-disable-next-line @typescript-eslint/no-unnecessary-condition -- see comment above
  return { prefix: DEFAULT_PHONE_PREFIX, localNumber: fullPhone ?? '' };
}

export function formatE164(prefix: string, localNumber: string): string {
  // `localNumber` is whatever the user typed in the input, which may include
  // spaces or dashes copied from the placeholder's "11 2345 6789" shape —
  // E.164 (and phoneField's regex) allows digits only, so those separators
  // must be stripped here rather than left to fail validation silently.
  return prefix + localNumber.replace(/\D/g, '');
}
