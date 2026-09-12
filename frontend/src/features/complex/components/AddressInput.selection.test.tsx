import { screen, fireEvent, waitFor, act } from '@testing-library/react';
import { vi, describe, it, expect, beforeEach, afterEach } from 'vitest';
import {
  mockAutocompleteSuccess,
  mockDetailsSuccess,
  mockDetailsJson,
  mockGet,
  typeAndFlush,
  setup,
} from './address-input-test-helpers';

describe('AddressInput selection', () => {
  beforeEach(() => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    vi.clearAllMocks();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('selects prediction and fetches place details on click', async () => {
    mockAutocompleteSuccess();
    mockDetailsSuccess();
    const { onChange, onSelect } = setup();

    await typeAndFlush(screen.getByRole('combobox'), 'Av. Libertador');

    await waitFor(() => {
      expect(screen.getByRole('listbox')).toBeInTheDocument();
    });

    const option = screen.getAllByRole('option')[0];
    if (option) fireEvent.mouseDown(option);

    // Immediately sets the description text.
    expect(onChange).toHaveBeenCalledWith('Av. Libertador 1234, Palermo, CABA, Argentina');

    // Dropdown closes.
    expect(screen.queryByRole('listbox')).not.toBeInTheDocument();

    // Fetches place details.
    await waitFor(() => {
      expect(mockGet).toHaveBeenCalledWith(
        'places/details',
        expect.objectContaining({
          searchParams: expect.objectContaining({ place_id: 'place_1' }) as unknown as Record<string, string>,
        }),
      );
    });

    // Calls onSelect with parsed details.
    await waitFor(() => {
      expect(onSelect).toHaveBeenCalledWith({
        address: 'Avenida del Libertador 1234',
        city: 'Buenos Aires',
        province: 'Ciudad Autónoma de Buenos Aires',
        formatted_address: 'Av. del Libertador 1234, C1425 CABA, Argentina',
        latitude: -34.574,
        longitude: -58.42,
      });
    });
  });

  it('handles details API failure gracefully', async () => {
    mockAutocompleteSuccess();
    mockDetailsJson.mockRejectedValue(new Error('Network error'));
    const { onChange, onSelect } = setup();

    await typeAndFlush(screen.getByRole('combobox'), 'Av. Libertador');

    await waitFor(() => {
      expect(screen.getByRole('listbox')).toBeInTheDocument();
    });

    const option = screen.getAllByRole('option')[0];
    if (option) fireEvent.mouseDown(option);

    // Still sets text even if details fail.
    expect(onChange).toHaveBeenCalledWith('Av. Libertador 1234, Palermo, CABA, Argentina');

    // onSelect is NOT called since details failed.
    await act(() => vi.advanceTimersByTimeAsync(100));
    expect(onSelect).not.toHaveBeenCalled();
  });
});
