// @vitest-environment node
import { describe, it, expect } from 'vitest';
import { generateCodeVerifier, generateCodeChallenge } from './pkce';

describe('generateCodeVerifier', () => {
  it('returns a 64-character hex string', () => {
    const verifier = generateCodeVerifier();
    expect(verifier).toHaveLength(64);
    expect(verifier).toMatch(/^[0-9a-f]+$/);
  });

  it('generates different values on each call', () => {
    const v1 = generateCodeVerifier();
    const v2 = generateCodeVerifier();
    expect(v1).not.toBe(v2);
  });
});

describe('generateCodeChallenge', () => {
  it('returns a non-empty base64url string', async () => {
    const verifier = generateCodeVerifier();
    const challenge = await generateCodeChallenge(verifier);
    expect(challenge.length).toBeGreaterThan(0);
  });

  it('uses base64url encoding (no +, /, or = characters)', async () => {
    const verifier = generateCodeVerifier();
    const challenge = await generateCodeChallenge(verifier);
    expect(challenge).not.toMatch(/[+/=]/);
  });

  it('produces the same challenge for the same verifier', async () => {
    const verifier = generateCodeVerifier();
    const challenge1 = await generateCodeChallenge(verifier);
    const challenge2 = await generateCodeChallenge(verifier);
    expect(challenge1).toBe(challenge2);
  });

  it('produces different challenges for different verifiers', async () => {
    const v1 = generateCodeVerifier();
    const v2 = generateCodeVerifier();
    const c1 = await generateCodeChallenge(v1);
    const c2 = await generateCodeChallenge(v2);
    expect(c1).not.toBe(c2);
  });
});
