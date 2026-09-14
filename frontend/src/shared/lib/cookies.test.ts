import { describe, it, expect, afterEach } from 'vitest';
import { readCookie } from './cookies';

function setCookie(entry: string) {
  document.cookie = entry;
}

afterEach(() => {
  for (const part of document.cookie.split(';')) {
    const name = part.trim().split('=')[0];
    if (name) document.cookie = `${name}=; max-age=0`;
  }
});

describe('readCookie', () => {
  it('reads a cookie by name', () => {
    setCookie('g_csrf_token=abc123');

    expect(readCookie('g_csrf_token')).toBe('abc123');
  });

  it('picks the right cookie out of several', () => {
    setCookie('first=one');
    setCookie('g_csrf_token=abc123');
    setCookie('last=two');

    expect(readCookie('g_csrf_token')).toBe('abc123');
    expect(readCookie('first')).toBe('one');
    expect(readCookie('last')).toBe('two');
  });

  it('returns undefined for a cookie that is not set', () => {
    setCookie('other=value');

    expect(readCookie('g_csrf_token')).toBeUndefined();
  });

  it('returns undefined when there are no cookies at all (e.g. cookies blocked)', () => {
    expect(readCookie('g_csrf_token')).toBeUndefined();
  });

  // A prefix match is not a name match: `not_g_csrf_token` and
  // `g_csrf_token_extra` are different cookies.
  it('does not match a cookie whose name merely contains the one asked for', () => {
    setCookie('not_g_csrf_token=wrong');
    setCookie('g_csrf_token_extra=alsowrong');

    expect(readCookie('g_csrf_token')).toBeUndefined();
  });

  it('returns undefined for a cookie set to an empty value', () => {
    setCookie('g_csrf_token=');

    expect(readCookie('g_csrf_token')).toBeUndefined();
  });

  it('decodes a percent-encoded value', () => {
    setCookie(`g_csrf_token=${encodeURIComponent('a+b/c=')}`);

    expect(readCookie('g_csrf_token')).toBe('a+b/c=');
  });

  // A lone `%` is not valid percent-encoding and makes decodeURIComponent
  // throw; the raw value is still what the server set, so it is handed back
  // rather than turned into a crash inside a read.
  it('falls back to the raw value when it is not valid percent-encoding', () => {
    setCookie('g_csrf_token=100%');

    expect(readCookie('g_csrf_token')).toBe('100%');
  });
});
