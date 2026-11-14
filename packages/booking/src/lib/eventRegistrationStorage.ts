import { z } from 'zod';
import { safeSessionStorage } from '@vibe/ui';

/**
 * The token of the spot being paid for, parked while the browser is away at the
 * checkout. A rejected payment returns without it (`booklink.EventFailure`
 * carries no token), and the held spot would block a second try with the same
 * phone until it expires, so the failure page releases it with this token.
 */
const key = (eventId: string) => `vibe_event_registration:${eventId}`;

export function storeEventRegistrationToken(eventId: string, token: string): void {
  safeSessionStorage.set(key(eventId), JSON.stringify(token));
}

export function readEventRegistrationToken(eventId: string): string | null {
  return safeSessionStorage.getJSON(key(eventId), z.string().min(1));
}

export function clearEventRegistrationToken(eventId: string): void {
  safeSessionStorage.remove(key(eventId));
}
