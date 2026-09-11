import { describe, it, expect, afterEach } from 'vitest';
import { renderHook } from '@testing-library/react';
import { usePageTitle, useCanonical, useStructuredData } from './usePageTitle';

describe('usePageTitle', () => {
  afterEach(() => {
    document.title = '';
  });

  it('sets document title with app name suffix', () => {
    renderHook(() => {
      usePageTitle('Reservas');
    });
    expect(document.title).toBe('Reservas - Vibe');
  });

  it('sets document title to app name when no title provided', () => {
    renderHook(() => {
      usePageTitle();
    });
    expect(document.title).toBe('Vibe');
  });

  it('sets document title to app name when title is undefined', () => {
    renderHook(() => {
      usePageTitle(undefined);
    });
    expect(document.title).toBe('Vibe');
  });

  it('resets document title on unmount', () => {
    const { unmount } = renderHook(() => {
      usePageTitle('Canchas');
    });
    expect(document.title).toBe('Canchas - Vibe');

    unmount();
    expect(document.title).toBe('Vibe');
  });

  it('updates title when title prop changes', () => {
    const { rerender } = renderHook(
      ({ title }) => {
        usePageTitle(title);
      },
      {
        initialProps: { title: 'Page A' },
      },
    );
    expect(document.title).toBe('Page A - Vibe');

    rerender({ title: 'Page B' });
    expect(document.title).toBe('Page B - Vibe');
  });
});

describe('useCanonical', () => {
  afterEach(() => {
    const el = document.querySelector('link[rel="canonical"]');
    if (el) el.remove();
  });

  it('creates a canonical link element', () => {
    renderHook(() => {
      useCanonical('https://example.com/page');
    });
    const el: HTMLLinkElement | null = document.querySelector('link[rel="canonical"]');
    if (!el) throw new Error('expected a canonical link element to be present');
    expect(el.href).toBe('https://example.com/page');
  });

  it('updates href when url changes', () => {
    const { rerender } = renderHook(
      ({ url }) => {
        useCanonical(url);
      },
      {
        initialProps: { url: 'https://example.com/a' },
      },
    );
    let el: HTMLLinkElement | null = document.querySelector('link[rel="canonical"]');
    if (!el) throw new Error('expected a canonical link element to be present');
    expect(el.href).toBe('https://example.com/a');

    rerender({ url: 'https://example.com/b' });
    // Re-query since cleanup may remove and re-create the element
    el = document.querySelector('link[rel="canonical"]');
    if (!el) throw new Error('expected a canonical link element to be present');
    expect(el.href).toBe('https://example.com/b');
  });

  it('removes canonical link on unmount', () => {
    const { unmount } = renderHook(() => {
      useCanonical('https://example.com');
    });
    expect(document.querySelector('link[rel="canonical"]')).not.toBeNull();

    unmount();
    expect(document.querySelector('link[rel="canonical"]')).toBeNull();
  });
});

describe('useStructuredData', () => {
  afterEach(() => {
    document.querySelectorAll('script[type="application/ld+json"]').forEach((el) => {
      el.remove();
    });
  });

  it('inserts a JSON-LD script tag', () => {
    const data = { '@type': 'SportsActivityLocation', name: 'Club Padel' };
    renderHook(() => {
      useStructuredData(data);
    });
    const scripts = document.querySelectorAll('script[type="application/ld+json"]');
    expect(scripts.length).toBe(1);
    expect(scripts[0]?.textContent).toBe(JSON.stringify(data));
  });

  it('does not insert a script tag when data is null', () => {
    renderHook(() => {
      useStructuredData(null);
    });
    const scripts = document.querySelectorAll('script[type="application/ld+json"]');
    expect(scripts.length).toBe(0);
  });

  it('removes script tag on unmount', () => {
    const data = { '@type': 'Organization', name: 'Test' };
    const { unmount } = renderHook(() => {
      useStructuredData(data);
    });
    expect(document.querySelectorAll('script[type="application/ld+json"]').length).toBe(1);

    unmount();
    expect(document.querySelectorAll('script[type="application/ld+json"]').length).toBe(0);
  });
});
