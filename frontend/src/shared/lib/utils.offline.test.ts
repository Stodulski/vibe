// Default environment (happy-dom, see vitest.config.ts) — `utils.test.ts`
// runs under `@vitest-environment node` and has no `navigator` to read, so
// the `!navigator.onLine` branch of getHttpErrorMessage lives here instead.
import { getHttpErrorMessage } from './utils';
import { ES_AR } from '@/shared/i18n/es_AR';

function setOnline(value: boolean) {
  Object.defineProperty(navigator, 'onLine', {
    value,
    writable: true,
    configurable: true,
  });
}

describe('getHttpErrorMessage while offline', () => {
  const originalOnLine = navigator.onLine;

  afterEach(() => {
    setOnline(originalOnLine);
  });

  it('returns the generic connectivity message when the browser knows it is offline, ignoring the caller fallback', () => {
    setOnline(false);
    // Not a ky error at all — e.g. a validation error thrown before any
    // request went out — but the browser is offline, which is the more
    // useful thing to tell the person here.
    expect(getHttpErrorMessage(new Error('unexpected'), 'Error generico')).toBe(ES_AR.common.networkError);
  });

  it('returns the caller fallback when online', () => {
    setOnline(true);
    expect(getHttpErrorMessage(new Error('unexpected'), 'Error generico')).toBe('Error generico');
  });
});
