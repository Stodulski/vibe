import { z } from 'zod';
import { safeSessionStorage } from '@vibe/ui';

/**
 * The token of the spot being paid for, parked while the browser is away at the
 * checkout. A rejected payment returns without it (the checkout's failure link
 * carries no token), and the held spot would block a second try with the same
 * phone until it expires, so the failure page releases it with this token.
 * One store per kind of sign-up, keyed by the event or tournament being paid.
 */
export function createRegistrationTokenStorage(prefix: string) {
  const key = (id: string) => `${prefix}:${id}`;
  return {
    store: (id: string, token: string): void => {
      safeSessionStorage.set(key(id), JSON.stringify(token));
    },
    read: (id: string): string | null => safeSessionStorage.getJSON(key(id), z.string().min(1)),
    clear: (id: string): void => {
      safeSessionStorage.remove(key(id));
    },
  };
}
