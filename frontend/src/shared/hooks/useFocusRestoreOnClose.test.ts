import { renderHook } from '@testing-library/react';
import { useFocusRestoreOnClose } from './useFocusRestoreOnClose';

describe('useFocusRestoreOnClose', () => {
  it('returns null while never opened', () => {
    const { result } = renderHook(() => useFocusRestoreOnClose(false));
    expect(result.current).toBeNull();
  });

  it('snapshots the focused element the instant it opens', () => {
    const button = document.createElement('button');
    document.body.append(button);
    button.focus();

    const { result, rerender } = renderHook(({ open }) => useFocusRestoreOnClose(open), {
      initialProps: { open: false },
    });
    expect(result.current).toBeNull();

    rerender({ open: true });
    expect(result.current).toBe(button);

    button.remove();
  });

  it('keeps the snapshot after closing, so the caller can still restore it', () => {
    const button = document.createElement('button');
    document.body.append(button);
    button.focus();

    const { result, rerender } = renderHook(({ open }) => useFocusRestoreOnClose(open), {
      initialProps: { open: true },
    });
    expect(result.current).toBe(button);

    rerender({ open: false });
    expect(result.current).toBe(button);

    button.remove();
  });

  it('takes a fresh snapshot on the next open', () => {
    const first = document.createElement('button');
    const second = document.createElement('button');
    document.body.append(first, second);
    first.focus();

    const { result, rerender } = renderHook(({ open }) => useFocusRestoreOnClose(open), {
      initialProps: { open: true },
    });
    expect(result.current).toBe(first);

    rerender({ open: false });
    second.focus();
    rerender({ open: true });
    expect(result.current).toBe(second);

    first.remove();
    second.remove();
  });
});
