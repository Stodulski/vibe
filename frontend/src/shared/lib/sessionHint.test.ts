import { describe, it, expect, beforeEach } from 'vitest';
import { hasSessionHint, setSessionHint } from './sessionHint';

/**
 * The session hint is what tells the guest pages whether this browser has ever
 * signed in. The cookies are HttpOnly, so the page cannot read them; this flag
 * is the only thing it can read before the session check answers.
 */
describe('session hint', () => {
  beforeEach(() => {
    localStorage.clear();
  });

  it('is absent in a browser that has never signed in', () => {
    expect(hasSessionHint()).toBe(false);
  });

  it('is set while a session exists and cleared when it ends', () => {
    setSessionHint(true);
    expect(hasSessionHint()).toBe(true);

    setSessionHint(false);
    expect(hasSessionHint()).toBe(false);
  });
});
