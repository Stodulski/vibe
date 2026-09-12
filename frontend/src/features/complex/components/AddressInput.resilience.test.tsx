import { screen, fireEvent, waitFor, act } from '@testing-library/react';
import { vi, describe, it, expect, beforeEach, afterEach } from 'vitest';
import { mockAutocompleteJson, mockDetailsJson, setup } from './address-input-test-helpers';

// Finding M2: every autocomplete/details failure was swallowed by an empty
// `catch`, leaving the user staring at a spinner that vanished with no
// explanation.
describe('AddressInput error surfacing', () => {
  beforeEach(() => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    vi.clearAllMocks();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('surfaces an inline error when the autocomplete request fails', async () => {
    mockAutocompleteJson.mockRejectedValue(new Error('Network error'));
    setup();

    fireEvent.change(screen.getByRole('combobox'), { target: { value: 'Av. Libertador' } });
    await act(() => vi.advanceTimersByTimeAsync(550));

    await waitFor(() => {
      expect(screen.getByRole('alert')).toHaveTextContent('No se pudo buscar la dirección. Probá de nuevo.');
    });
  });

  it('surfaces an inline error when fetching place details fails', async () => {
    mockAutocompleteJson.mockResolvedValue({
      predictions: [
        {
          place_id: 'place_1',
          description: 'Av. Libertador 1234, Palermo, CABA, Argentina',
          structured_formatting: {
            main_text: 'Av. Libertador 1234',
            secondary_text: 'Palermo, CABA, Argentina',
          },
        },
      ],
    });
    mockDetailsJson.mockRejectedValue(new Error('Network error'));
    setup();

    fireEvent.change(screen.getByRole('combobox'), { target: { value: 'Av. Libertador' } });
    await act(() => vi.advanceTimersByTimeAsync(550));

    await waitFor(() => {
      expect(screen.getByRole('listbox')).toBeInTheDocument();
    });
    const option = screen.getAllByRole('option')[0];
    if (option) fireEvent.mouseDown(option);

    await waitFor(() => {
      expect(screen.getByRole('alert')).toHaveTextContent(
        'No se pudo obtener la dirección seleccionada. Probá de nuevo.',
      );
    });
  });
});
