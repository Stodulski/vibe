import { describe, it, expect, afterEach } from 'vitest';
import { renderHook } from '@testing-library/react';
import { usePageTitle } from './usePageTitle';

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
