import { afterEach, describe, expect, it, vi } from 'vitest';
import { linkWebManifest, shouldRegisterServiceWorker, unregisterServiceWorkers } from './serviceWorkerHost';

const APP_URL = 'https://app.vibe.com.ar';

describe('shouldRegisterServiceWorker', () => {
  it('registers on the app host', () => {
    expect(shouldRegisterServiceWorker('app.vibe.com.ar', APP_URL)).toBe(true);
  });

  it('never registers on the landing host', () => {
    expect(shouldRegisterServiceWorker('vibe.com.ar', APP_URL)).toBe(false);
  });

  it('never registers on the www alias of the landing host', () => {
    expect(shouldRegisterServiceWorker('www.vibe.com.ar', APP_URL)).toBe(false);
  });

  it('never registers on a hostname that merely starts with the app host', () => {
    expect(shouldRegisterServiceWorker('app.vibe.com.ar.evil.example', APP_URL)).toBe(false);
  });

  it('registers on localhost and 127.0.0.1 whatever the app URL says', () => {
    expect(shouldRegisterServiceWorker('localhost', APP_URL)).toBe(true);
    expect(shouldRegisterServiceWorker('127.0.0.1', APP_URL)).toBe(true);
    expect(shouldRegisterServiceWorker('localhost', '')).toBe(true);
    expect(shouldRegisterServiceWorker('127.0.0.1', 'not a url')).toBe(true);
  });

  it('compares hostnames only, ignoring the scheme, port and path of the app URL', () => {
    expect(shouldRegisterServiceWorker('app.vibe.com.ar', 'http://app.vibe.com.ar:8080/dashboard')).toBe(true);
  });

  it('fails closed when the app URL is empty or malformed', () => {
    expect(shouldRegisterServiceWorker('app.vibe.com.ar', '')).toBe(false);
    expect(shouldRegisterServiceWorker('app.vibe.com.ar', 'not a url')).toBe(false);
    expect(shouldRegisterServiceWorker('vibe.com.ar', 'not a url')).toBe(false);
  });

  it('fails closed when a hostname is empty', () => {
    expect(shouldRegisterServiceWorker('', APP_URL)).toBe(false);
    expect(shouldRegisterServiceWorker('', 'file:///index.html')).toBe(false);
  });
});

describe('unregisterServiceWorkers', () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  function stubServiceWorker(getRegistrations: () => Promise<unknown>) {
    vi.stubGlobal('navigator', { serviceWorker: { getRegistrations } });
  }

  /** A registration whose only worker runs `scriptPath`, in the given state. */
  function registrationRunning(scriptPath: string, state: 'active' | 'waiting' | 'installing' = 'active') {
    const worker = { scriptURL: `https://vibe.com.ar${scriptPath}` };
    return {
      active: state === 'active' ? worker : null,
      waiting: state === 'waiting' ? worker : null,
      installing: state === 'installing' ? worker : null,
      unregister: vi.fn().mockResolvedValue(true),
    };
  }

  it("unregisters this app's worker (/sw.js)", async () => {
    const appWorker = registrationRunning('/sw.js');
    stubServiceWorker(vi.fn().mockResolvedValue([appWorker]));

    await unregisterServiceWorkers();

    expect(appWorker.unregister).toHaveBeenCalledTimes(1);
  });

  it('leaves a registration running another script alone', async () => {
    const appWorker = registrationRunning('/sw.js');
    const otherWorker = registrationRunning('/landing-sw.js');
    stubServiceWorker(vi.fn().mockResolvedValue([appWorker, otherWorker]));

    await unregisterServiceWorkers();

    expect(appWorker.unregister).toHaveBeenCalledTimes(1);
    expect(otherWorker.unregister).not.toHaveBeenCalled();
  });

  it('matches this app worker while it is waiting or installing, not only when active', async () => {
    const waiting = registrationRunning('/sw.js', 'waiting');
    const installing = registrationRunning('/sw.js', 'installing');
    stubServiceWorker(vi.fn().mockResolvedValue([waiting, installing]));

    await unregisterServiceWorkers();

    expect(waiting.unregister).toHaveBeenCalledTimes(1);
    expect(installing.unregister).toHaveBeenCalledTimes(1);
  });

  it('swallows a failed registration lookup', async () => {
    stubServiceWorker(() => Promise.reject(new Error('SecurityError')));

    await expect(unregisterServiceWorkers()).resolves.toBeUndefined();
  });

  it('swallows a failed unregister', async () => {
    const appWorker = registrationRunning('/sw.js');
    appWorker.unregister.mockRejectedValue(new Error('boom'));
    stubServiceWorker(vi.fn().mockResolvedValue([appWorker]));

    await expect(unregisterServiceWorkers()).resolves.toBeUndefined();
    expect(appWorker.unregister).toHaveBeenCalledTimes(1);
  });

  it('does nothing where service workers are unsupported', async () => {
    vi.stubGlobal('navigator', {});

    await expect(unregisterServiceWorkers()).resolves.toBeUndefined();
  });
});

describe('linkWebManifest', () => {
  /** A detached document, so the links never leak into the test page itself. */
  function freshDocument(): Document {
    return document.implementation.createHTMLDocument('');
  }

  function manifestLinks(doc: Document) {
    return doc.head.querySelectorAll('link[rel="manifest"]');
  }

  it('links the manifest once on the app host, however often it is called', () => {
    const doc = freshDocument();

    linkWebManifest('app.vibe.com.ar', APP_URL, doc);
    linkWebManifest('app.vibe.com.ar', APP_URL, doc);

    const links = manifestLinks(doc);
    expect(links).toHaveLength(1);
    expect(links[0]?.getAttribute('href')).toBe('/manifest.json');
  });

  it('appends nothing on the landing host', () => {
    const doc = freshDocument();

    linkWebManifest('vibe.com.ar', APP_URL, doc);
    linkWebManifest('www.vibe.com.ar', APP_URL, doc);

    expect(manifestLinks(doc)).toHaveLength(0);
  });

  it('links the manifest on localhost, where the worker runs too', () => {
    const doc = freshDocument();

    linkWebManifest('localhost', APP_URL, doc);

    expect(manifestLinks(doc)).toHaveLength(1);
  });
});
