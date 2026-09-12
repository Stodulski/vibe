import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import type { GoogleNamespace } from './googleIdentity';

function stubGoogle(): GoogleNamespace {
  const google: GoogleNamespace = {
    accounts: { id: { initialize: vi.fn(), renderButton: vi.fn() } },
  };
  window.google = google;
  return google;
}

describe('loadGoogleIdentityServices', () => {
  let appendedScripts: HTMLScriptElement[];

  // `loadPromise` is module-scoped state (the single-flight cache) — a fresh
  // module per test via `vi.resetModules()` + a dynamic re-import is the
  // only way to get a clean slate between tests, the same way it's not
  // exposed for production callers to reset either.
  beforeEach(() => {
    vi.resetModules();
    delete window.google;
    appendedScripts = [];

    // happy-dom attempts a real network fetch for any <script src> actually
    // inserted into the document — disabled in this test environment, which
    // makes it dispatch a synchronous 'error' event of its own before a test
    // gets to drive anything. Stubbed so these tests can fire load/error on
    // the script directly instead of racing happy-dom's own attempt.
    vi.spyOn(document.head, 'appendChild').mockImplementation((node) => {
      appendedScripts.push(node as HTMLScriptElement);
      return node;
    });
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  function lastAppendedScript(): HTMLScriptElement | undefined {
    return appendedScripts.at(-1);
  }

  it('injects the script into <head> exactly once, even across concurrent calls', async () => {
    const { loadGoogleIdentityServices } = await import('./googleIdentity');

    void loadGoogleIdentityServices();
    void loadGoogleIdentityServices();

    expect(appendedScripts).toHaveLength(1);
    expect(appendedScripts[0]?.src).toBe('https://accounts.google.com/gsi/client');
  });

  it('resolves with window.google once the script fires its load event', async () => {
    const { loadGoogleIdentityServices } = await import('./googleIdentity');

    const promise = loadGoogleIdentityServices();
    const google = stubGoogle();

    lastAppendedScript()?.dispatchEvent(new Event('load'));

    await expect(promise).resolves.toBe(google);
  });

  it('rejects when the script fails to load', async () => {
    const { loadGoogleIdentityServices } = await import('./googleIdentity');

    const promise = loadGoogleIdentityServices();

    lastAppendedScript()?.dispatchEvent(new Event('error'));

    await expect(promise).rejects.toThrow('Failed to load the Google Identity Services script');
  });

  it('resolves immediately without injecting a script when window.google is already present', async () => {
    const { loadGoogleIdentityServices } = await import('./googleIdentity');
    const google = stubGoogle();

    await expect(loadGoogleIdentityServices()).resolves.toBe(google);
    expect(appendedScripts).toHaveLength(0);
  });

  it('retries with a fresh script tag after a previous load failed', async () => {
    const { loadGoogleIdentityServices } = await import('./googleIdentity');

    const firstAttempt = loadGoogleIdentityServices();
    const firstScript = lastAppendedScript();
    firstScript?.dispatchEvent(new Event('error'));
    await expect(firstAttempt).rejects.toThrow();

    const secondAttempt = loadGoogleIdentityServices();
    const secondScript = lastAppendedScript();
    expect(secondScript).not.toBeUndefined();
    expect(secondScript).not.toBe(firstScript);

    const google = stubGoogle();
    secondScript?.dispatchEvent(new Event('load'));

    await expect(secondAttempt).resolves.toBe(google);
  });
});
