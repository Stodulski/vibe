import { parseEnv } from './env';

describe('parseEnv', () => {
  it('applies the /api/v1 default when VITE_API_URL is missing', () => {
    const result = parseEnv({} as never);
    expect(result.VITE_API_URL).toBe('/api/v1');
  });

  it('keeps an explicit VITE_API_URL', () => {
    const result = parseEnv({ VITE_API_URL: 'https://api.example.com' } as never);
    expect(result.VITE_API_URL).toBe('https://api.example.com');
  });

  it('rejects a malformed VITE_APP_URL', () => {
    expect(() => parseEnv({ VITE_APP_URL: 'not-a-url' } as never)).toThrow(/VITE_APP_URL/);
  });

  it('accepts a valid VITE_APP_URL', () => {
    const result = parseEnv({ VITE_APP_URL: 'https://app.example.com' } as never);
    expect(result.VITE_APP_URL).toBe('https://app.example.com');
  });
});
