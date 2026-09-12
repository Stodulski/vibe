import { parseEnv } from './env';

const APP_URL = 'https://app.example.com';

describe('parseEnv', () => {
  it('applies the /api/v1 default when VITE_API_URL is missing', () => {
    const result = parseEnv({ VITE_APP_URL: APP_URL } as never);
    expect(result.VITE_API_URL).toBe('/api/v1');
  });

  it('keeps an explicit VITE_API_URL', () => {
    const result = parseEnv({ VITE_API_URL: 'https://api.example.com', VITE_APP_URL: APP_URL } as never);
    expect(result.VITE_API_URL).toBe('https://api.example.com');
  });

  it('rejects a malformed VITE_APP_URL', () => {
    expect(() => parseEnv({ VITE_APP_URL: 'not-a-url' } as never)).toThrow(/VITE_APP_URL/);
  });

  it('accepts a valid VITE_APP_URL', () => {
    const result = parseEnv({ VITE_APP_URL: APP_URL } as never);
    expect(result.VITE_APP_URL).toBe(APP_URL);
  });

  // BLD-04: VITE_APP_URL is required so a deploy missing it fails the build
  // instead of silently falling back to window.location.origin.
  it('rejects a missing VITE_APP_URL', () => {
    expect(() => parseEnv({} as never)).toThrow(/VITE_APP_URL/);
  });
});
