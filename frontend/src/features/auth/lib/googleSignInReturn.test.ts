import { describe, it, expect, vi, afterEach } from 'vitest';
import { rememberGoogleReturnPath, takeGoogleReturnPath } from './googleSignInReturn';

const KEY = 'vibe.google-signin.from';

afterEach(() => {
  window.sessionStorage.clear();
  vi.restoreAllMocks();
});

describe('googleSignInReturn', () => {
  it('gives back the destination it was handed', () => {
    rememberGoogleReturnPath('/bookings?date=2026-03-18');

    expect(takeGoogleReturnPath()).toBe('/bookings?date=2026-03-18');
  });

  // The code it travels with is single-use, and so is this: a second read
  // belongs to a different sign-in, which parks its own destination or has
  // none at all.
  it('removes the destination as it reads it', () => {
    rememberGoogleReturnPath('/complexes/abc/bookings');

    expect(takeGoogleReturnPath()).toBe('/complexes/abc/bookings');
    expect(takeGoogleReturnPath()).toBeUndefined();
    expect(window.sessionStorage.getItem(KEY)).toBeNull();
  });

  it('returns undefined when nothing was ever remembered', () => {
    expect(takeGoogleReturnPath()).toBeUndefined();
  });

  it.each([
    ['an absolute URL elsewhere', 'https://evil.example/steal'],
    ['a protocol-relative URL', '//evil.example'],
    ['a path outside the allowlist', '/not-a-real-section'],
  ])('refuses to remember %s', (_label, candidate) => {
    rememberGoogleReturnPath(candidate);

    expect(window.sessionStorage.getItem(KEY)).toBeNull();
    expect(takeGoogleReturnPath()).toBeUndefined();
  });

  // The dangerous case: an attempt that was abandoned leaves a destination
  // behind, and the next sign-in — which had none of its own — would inherit
  // it and send the person somewhere they never asked for.
  it('clears a previously remembered destination when there is nothing to remember', () => {
    rememberGoogleReturnPath('/bookings');
    rememberGoogleReturnPath(undefined);

    expect(takeGoogleReturnPath()).toBeUndefined();
  });

  it('clears a previously remembered destination when the new one is unsafe', () => {
    rememberGoogleReturnPath('/bookings');
    rememberGoogleReturnPath('https://evil.example/steal');

    expect(takeGoogleReturnPath()).toBeUndefined();
  });

  // Not localStorage: a destination parked in one tab must not steer a
  // sign-in started in another one tomorrow.
  it('stores nothing in localStorage', () => {
    rememberGoogleReturnPath('/bookings');

    expect(window.localStorage.getItem(KEY)).toBeNull();
    expect(window.sessionStorage.getItem(KEY)).toBe('/bookings');
  });
});

// A sibling describe, not nested: max-lines-per-function counts a describe
// callback's whole body.
describe('googleSignInReturn — storage that refuses', () => {
  // Storage rejected outright is what Safari private browsing and a locked
  // down corporate profile look like. Losing the destination there is a worse
  // redirect, never a crash on the way into sign-in.
  it('survives a sessionStorage that throws on write', () => {
    vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => {
      throw new Error('QuotaExceededError');
    });

    expect(() => {
      rememberGoogleReturnPath('/bookings');
    }).not.toThrow();
  });

  it('survives a sessionStorage that throws on read', () => {
    vi.spyOn(Storage.prototype, 'getItem').mockImplementation(() => {
      throw new Error('SecurityError');
    });

    expect(takeGoogleReturnPath()).toBeUndefined();
  });

  it('survives a sessionStorage that throws on removal', () => {
    vi.spyOn(Storage.prototype, 'removeItem').mockImplementation(() => {
      throw new Error('SecurityError');
    });

    expect(() => {
      rememberGoogleReturnPath(undefined);
    }).not.toThrow();
    expect(() => takeGoogleReturnPath()).not.toThrow();
  });
});
