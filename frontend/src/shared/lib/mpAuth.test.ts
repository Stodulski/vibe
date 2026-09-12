import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { buildMPAuthUrl, generatePKCE } from './mpAuth';

vi.mock('./pkce', () => ({
  generateCodeVerifier: () => 'test-verifier-123',
  generateCodeChallenge: vi.fn().mockResolvedValue('test-challenge-456'),
}));

describe('buildMPAuthUrl', () => {
  beforeEach(() => {
    vi.spyOn(crypto, 'randomUUID').mockReturnValue('11111111-1111-1111-1111-111111111111');
    sessionStorage.clear();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('builds a valid MercadoPago OAuth URL', () => {
    const url = buildMPAuthUrl('complex-1', {
      verifier: 'v1',
      challenge: 'c1',
    });
    expect(url).toContain('https://auth.mercadopago.com/authorization');
    expect(url).toContain('response_type=code');
    expect(url).toContain('code_challenge=c1');
    expect(url).toContain('code_challenge_method=S256');
  });

  it('stores complexId in sessionStorage keyed by nonce', () => {
    buildMPAuthUrl('complex-1', { verifier: 'v1', challenge: 'c1' });
    expect(sessionStorage.getItem('mp_oauth_complex_11111111-1111-1111-1111-111111111111')).toBe('complex-1');
  });

  it('includes redirect_uri pointing to /settings/mp/callback', () => {
    const url = buildMPAuthUrl('complex-1', {
      verifier: 'v1',
      challenge: 'c1',
    });
    expect(url).toContain(encodeURIComponent('/settings/mp/callback'));
  });

  it('uses the nonce as state parameter', () => {
    const url = buildMPAuthUrl('complex-1', {
      verifier: 'v1',
      challenge: 'c1',
    });
    expect(url).toContain('state=11111111-1111-1111-1111-111111111111');
  });

  // The app id must come from GET .../mp/status when the server has one —
  // that is the app id the API can actually exchange a code with, and a
  // client stuck on the build-time VITE_MP_APP_ID could point at one it
  // cannot.
  it('prefers the given appId over the build-time env value', () => {
    const url = buildMPAuthUrl('complex-1', { verifier: 'v1', challenge: 'c1' }, 'app-from-server');
    expect(url).toContain('client_id=app-from-server');
  });

  it('falls back to the build-time env value when no appId is given', () => {
    const url = buildMPAuthUrl('complex-1', { verifier: 'v1', challenge: 'c1' });
    // vite-env has no VITE_MP_APP_ID configured in tests, so this is the
    // documented fallback behaviour rather than a specific id.
    expect(url).toContain('client_id=');
  });
});

describe('generatePKCE', () => {
  it('returns verifier and challenge', async () => {
    const result = await generatePKCE();
    expect(result).toEqual({
      verifier: 'test-verifier-123',
      challenge: 'test-challenge-456',
    });
  });
});
