import { describe, it, expect } from 'vitest';
import { act, render } from '@testing-library/react';
import { useStore } from './index';

describe('an atomic store selector', () => {
  it('still re-renders when the selected value itself changes', () => {
    let renders = 0;
    function CsrfToken() {
      useStore((s) => s.csrfToken);
      renders += 1;
      return null;
    }
    render(<CsrfToken />);
    const before = renders;

    act(() => {
      useStore.getState().setCsrfToken('csrf-10');
    });

    expect(renders).toBeGreaterThan(before);
  });
});
