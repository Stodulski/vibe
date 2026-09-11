import { screen, fireEvent, act } from '@testing-library/react';
import { vi } from 'vitest';
import { mockAutocompleteJson, mockGet, setup } from './address-input-test-helpers';

// Finding M2: the debounce timer was never cleared on unmount, and no
// `AbortController` was passed to the underlying request.
describe('AddressInput unmount cleanup', () => {
  beforeEach(() => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    vi.clearAllMocks();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('does not fire a request if the component unmounts before the debounce elapses', async () => {
    const { unmount } = setup();
    fireEvent.change(screen.getByRole('combobox'), { target: { value: 'Av. Libertador' } });

    unmount();
    await act(() => vi.advanceTimersByTimeAsync(550));

    expect(mockGet).not.toHaveBeenCalled();
  });

  it('aborts the pending predictions request on unmount', async () => {
    const abortSpy = vi.spyOn(AbortController.prototype, 'abort');
    mockAutocompleteJson.mockImplementation(
      () =>
        new Promise(() => {
          /* never resolves */
        }),
    );

    const { unmount } = setup();
    fireEvent.change(screen.getByRole('combobox'), { target: { value: 'Av. Libertador' } });
    await act(() => vi.advanceTimersByTimeAsync(550));

    expect(abortSpy).not.toHaveBeenCalled();
    unmount();
    expect(abortSpy).toHaveBeenCalledTimes(1);

    abortSpy.mockRestore();
  });
});
