import { describe, it, expect, beforeEach } from 'vitest';
import { clearMPOAuthSessions, consumeMPOAuthSession, saveMPOAuthSession } from './mpOAuthSession';

const session = (n: number) => ({ complexId: `c${String(n)}`, codeVerifier: `v${String(n)}`, returnPath: '/settings' });
const MINUTE = 60 * 1000;

describe('mpOAuthSession', () => {
  beforeEach(() => {
    sessionStorage.clear();
  });

  it('keeps one entry per state, each with its own verifier', () => {
    saveMPOAuthSession('state-a', session(1));
    saveMPOAuthSession('state-b', session(2));

    expect(consumeMPOAuthSession('state-b')?.codeVerifier).toBe('v2');
    expect(consumeMPOAuthSession('state-a')?.codeVerifier).toBe('v1');
  });

  it('removes the entry as it reads it, so a second read finds nothing', () => {
    saveMPOAuthSession('state-a', session(1));

    expect(consumeMPOAuthSession('state-a')).not.toBeNull();
    expect(consumeMPOAuthSession('state-a')).toBeNull();
    expect(sessionStorage.length).toBe(0);
  });

  it('returns null for an unknown state and leaves other entries alone', () => {
    saveMPOAuthSession('state-a', session(1));

    expect(consumeMPOAuthSession('nope')).toBeNull();
    expect(consumeMPOAuthSession('state-a')).not.toBeNull();
  });

  it('drops and rejects an unreadable entry', () => {
    sessionStorage.setItem('mp_oauth_bad', '{not json');
    sessionStorage.setItem('mp_oauth_wrong-shape', JSON.stringify({ complexId: 'c1' }));

    expect(consumeMPOAuthSession('bad')).toBeNull();
    expect(consumeMPOAuthSession('wrong-shape')).toBeNull();
    expect(sessionStorage.length).toBe(0);
  });

  it.each([
    '//evil.com',
    '/\\evil.com',
    '/\t/evil.com',
    '/%2Fevil.com',
    '/%5Cevil.com',
    'https://evil.com',
    'javascript:alert(1)',
    'settings',
    '',
  ])('rejects the return path %j', (returnPath) => {
    sessionStorage.setItem('mp_oauth_evil', JSON.stringify({ ...session(1), returnPath, createdAt: Date.now() }));
    expect(consumeMPOAuthSession('evil')).toBeNull();
  });

  it.each(['/settings', '/onboarding', '/settings?tab=billing'])('accepts the same-origin path %j', (returnPath) => {
    sessionStorage.setItem('mp_oauth_ok', JSON.stringify({ ...session(1), returnPath, createdAt: Date.now() }));
    expect(consumeMPOAuthSession('ok')?.returnPath).toBe(returnPath);
  });

  it('rejects and deletes an entry older than ten minutes on consume', () => {
    const now = Date.now();
    saveMPOAuthSession('stale', session(1), now - 11 * MINUTE);
    saveMPOAuthSession('edge', session(2), now - 10 * MINUTE);

    expect(consumeMPOAuthSession('stale', now)).toBeNull();
    expect(sessionStorage.getItem('mp_oauth_stale')).toBeNull();
    expect(consumeMPOAuthSession('edge', now)?.codeVerifier).toBe('v2');
  });
});

describe('mpOAuthSession pruning', () => {
  beforeEach(() => {
    sessionStorage.clear();
  });

  it('prunes entries older than ten minutes when writing', () => {
    const now = Date.now();
    saveMPOAuthSession('old', session(1), now - 11 * MINUTE);
    saveMPOAuthSession('fresh', session(2), now - 2 * MINUTE);

    saveMPOAuthSession('new', session(3), now);

    expect(sessionStorage.getItem('mp_oauth_old')).toBeNull();
    expect(sessionStorage.getItem('mp_oauth_fresh')).not.toBeNull();
    expect(sessionStorage.getItem('mp_oauth_new')).not.toBeNull();
  });

  it('caps the number of entries kept, dropping the oldest', () => {
    const now = Date.now();
    for (let i = 0; i < 8; i += 1) saveMPOAuthSession(`s${String(i)}`, session(i), now + i);

    const kept = Object.keys(sessionStorage).filter((k) => k.startsWith('mp_oauth_'));
    expect(kept).toHaveLength(5);
    expect(sessionStorage.getItem('mp_oauth_s7')).not.toBeNull();
    expect(sessionStorage.getItem('mp_oauth_s0')).toBeNull();
  });

  it('sweeps the legacy single-verifier and nonce-to-complexId keys on write', () => {
    sessionStorage.setItem('mp_code_verifier', 'legacy');
    sessionStorage.setItem('mp_return_path', '/settings');
    sessionStorage.setItem('mp_oauth_complex_abc', 'c1');

    saveMPOAuthSession('state-a', session(1));

    expect(sessionStorage.getItem('mp_code_verifier')).toBeNull();
    expect(sessionStorage.getItem('mp_return_path')).toBeNull();
    expect(sessionStorage.getItem('mp_oauth_complex_abc')).toBeNull();
  });

  it('never reads a legacy complexId-only entry as an attempt', () => {
    sessionStorage.setItem('mp_oauth_complex_abc', 'c1');
    expect(consumeMPOAuthSession('complex_abc')).toBeNull();
  });

  it('clearMPOAuthSessions removes every entry and the legacy keys', () => {
    saveMPOAuthSession('state-a', session(1));
    sessionStorage.setItem('mp_code_verifier', 'legacy');
    sessionStorage.setItem('unrelated', 'keep');

    clearMPOAuthSessions();

    expect(sessionStorage.getItem('mp_oauth_state-a')).toBeNull();
    expect(sessionStorage.getItem('mp_code_verifier')).toBeNull();
    expect(sessionStorage.getItem('unrelated')).toBe('keep');
  });
});
