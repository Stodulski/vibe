import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { buildMPAuthUrl, createMPAuthAttempt } from './mpAuth';

vi.mock('./pkce', () => ({
  generateCodeVerifier: () => 'test-verifier-123',
  generateCodeChallenge: vi.fn().mockResolvedValue('test-challenge-456'),
}));

const attempt = { state: '11111111-1111-1111-1111-111111111111', verifier: 'v1', challenge: 'c1' };

describe('buildMPAuthUrl', () => {
  beforeEach(() => {
    sessionStorage.clear();
  });

  it('builds a valid MercadoPago OAuth URL', () => {
    const url = buildMPAuthUrl(attempt);
    expect(url).toContain('https://auth.mercadopago.com/authorization');
    expect(url).toContain('response_type=code');
    expect(url).toContain('code_challenge=c1');
    expect(url).toContain('code_challenge_method=S256');
  });

  // Persisting is the click handler's job (saveMPOAuthSession): building the
  // URL runs on every status load and must leave nothing behind.
  it('is pure: it writes nothing to sessionStorage', () => {
    buildMPAuthUrl(attempt);
    expect(sessionStorage.length).toBe(0);
  });

  it('includes redirect_uri pointing to /settings/mp/callback', () => {
    const url = buildMPAuthUrl(attempt);
    expect(url).toContain(encodeURIComponent('/settings/mp/callback'));
  });

  it('uses the attempt nonce as state parameter', () => {
    const url = buildMPAuthUrl(attempt);
    expect(url).toContain('state=11111111-1111-1111-1111-111111111111');
  });

  // The app id must come from GET .../mp/status when the server has one —
  // that is the app id the API can actually exchange a code with, and a
  // client stuck on the build-time VITE_MP_APP_ID could point at one it
  // cannot.
  it('prefers the given appId over the build-time env value', () => {
    const url = buildMPAuthUrl(attempt, 'app-from-server');
    expect(url).toContain('client_id=app-from-server');
  });

  it('falls back to the build-time env value when no appId is given', () => {
    const url = buildMPAuthUrl(attempt);
    // vite-env has no VITE_MP_APP_ID configured in tests, so this is the
    // documented fallback behaviour rather than a specific id.
    expect(url).toContain('client_id=');
  });
});

describe('createMPAuthAttempt', () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('returns verifier, challenge and a fresh state nonce', async () => {
    vi.spyOn(crypto, 'randomUUID').mockReturnValue('22222222-2222-2222-2222-222222222222');
    const result = await createMPAuthAttempt();
    expect(result).toEqual({
      state: '22222222-2222-2222-2222-222222222222',
      verifier: 'test-verifier-123',
      challenge: 'test-challenge-456',
    });
  });

  it('uses a different state for every attempt', async () => {
    const a = await createMPAuthAttempt();
    const b = await createMPAuthAttempt();
    expect(a.state).not.toBe(b.state);
  });
});
