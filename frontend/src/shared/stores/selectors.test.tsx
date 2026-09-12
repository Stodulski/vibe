import { describe, it, expect } from 'vitest';
import { act, render } from '@testing-library/react';
import { useStore } from './index';

/**
 * STORE-02: eleven call sites used to destructure the whole store
 * (`const { logout } = useStore()`), which subscribes the component to every
 * slice. Selecting a complex re-rendered the sidebar, the profile form and
 * every hook that only ever wanted `logout`. The ESLint rule in
 * `eslint.config.js` keeps the bare form out; this proves what the selector
 * buys.
 */
function countRenders() {
  let renders = 0;
  function OnlyLogout() {
    useStore((s) => s.logout);
    renders += 1;
    return null;
  }
  const view = render(<OnlyLogout />);
  return { renders: () => renders, view };
}

describe('an atomic store selector', () => {
  it('does not re-render its component when an unrelated slice changes', () => {
    const { renders } = countRenders();
    const before = renders();

    act(() => {
      useStore.getState().setSelectedComplexId('complex-9');
    });

    expect(useStore.getState().selectedComplexId).toBe('complex-9');
    expect(renders()).toBe(before);
  });

  it('still re-renders when the selected value itself changes', () => {
    let renders = 0;
    function SelectedComplex() {
      useStore((s) => s.selectedComplexId);
      renders += 1;
      return null;
    }
    render(<SelectedComplex />);
    const before = renders;

    act(() => {
      useStore.getState().setSelectedComplexId('complex-10');
    });

    expect(renders).toBeGreaterThan(before);
  });
});
