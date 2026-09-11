import { screen, fireEvent, waitFor, act } from '@testing-library/react';
import { vi } from 'vitest';
import { mockAutocompleteJson, mockGet, setup } from './address-input-test-helpers';

// Finding M2: a slow response for an earlier query could land after a faster
// response for a later one, because no `AbortController` tied the request to
// the search that started it.
// Hoisted to file scope (rather than nested in the describe below) partly to
// keep the describe callback's own line count under the repo's
// max-lines-per-function cap, which counts a nested `beforeEach`/`afterEach`'s
// lines as part of it.
beforeEach(() => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  vi.clearAllMocks();
});

afterEach(() => {
  vi.useRealTimers();
});

// A prediction fixture shaped like the Places API response, to keep the two
// nearly-identical ones below from padding out the test's own line count.
function prediction(placeId: string, mainText: string, secondaryText: string) {
  return {
    place_id: placeId,
    description: `${mainText}, ${secondaryText}`,
    structured_formatting: { main_text: mainText, secondary_text: secondaryText },
  };
}

describe('AddressInput out-of-order responses', () => {
  it('ignores a stale response that resolves after a newer search superseded it', async () => {
    let resolveFirst!: (value: unknown) => void;
    let resolveSecond!: (value: unknown) => void;
    mockAutocompleteJson
      .mockImplementationOnce(
        () =>
          new Promise((resolve) => {
            resolveFirst = resolve;
          }),
      )
      .mockImplementationOnce(
        () =>
          new Promise((resolve) => {
            resolveSecond = resolve;
          }),
      );

    setup();
    const input = screen.getByRole('combobox');

    fireEvent.change(input, { target: { value: 'Av. Lib' } });
    await act(() => vi.advanceTimersByTimeAsync(550));

    fireEvent.change(input, { target: { value: 'Av. Libertador' } });
    await act(() => vi.advanceTimersByTimeAsync(550));

    expect(mockGet).toHaveBeenCalledTimes(2);

    // The newer ("Av. Libertador") request resolves first.
    act(() => {
      resolveSecond({
        predictions: [prediction('place_2', 'Av. Libertador 5678', 'Vicente Lopez, Buenos Aires, Argentina')],
      });
    });
    await waitFor(() => {
      expect(screen.getByRole('listbox')).toBeInTheDocument();
    });

    // The stale ("Av. Lib") request resolves late, for a query that was
    // superseded and aborted — it must not overwrite what is on screen.
    act(() => {
      resolveFirst({
        predictions: [prediction('place_1', 'Av. Libertador 1234', 'Palermo, CABA, Argentina')],
      });
    });

    const options = screen.getAllByRole('option');
    expect(options).toHaveLength(1);
    expect(options[0]).toHaveTextContent('Av. Libertador 5678');
  });
});
