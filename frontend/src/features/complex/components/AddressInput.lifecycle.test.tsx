import { screen, fireEvent, waitFor, act } from '@testing-library/react';
import { vi, describe, it, expect, beforeEach, afterEach } from 'vitest';
import {
  mockAutocompleteSuccess,
  mockAutocompleteJson,
  mockGet,
  typeAndFlush,
  setup,
} from './address-input-test-helpers';

describe('AddressInput lifecycle', () => {
  beforeEach(() => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    vi.clearAllMocks();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('closes dropdown on outside click', async () => {
    mockAutocompleteSuccess();
    setup();

    await typeAndFlush(screen.getByRole('combobox'), 'Av. Libertador');

    await waitFor(() => {
      expect(screen.getByRole('listbox')).toBeInTheDocument();
    });

    fireEvent.mouseDown(document.body);
    expect(screen.queryByRole('listbox')).not.toBeInTheDocument();
  });

  it('reopens dropdown on focus if predictions exist', async () => {
    mockAutocompleteSuccess();
    setup();
    const input = screen.getByRole('combobox');

    await typeAndFlush(input, 'Av. Libertador');

    await waitFor(() => {
      expect(screen.getByRole('listbox')).toBeInTheDocument();
    });

    fireEvent.keyDown(input, { key: 'Escape' });
    expect(screen.queryByRole('listbox')).not.toBeInTheDocument();

    fireEvent.focus(input);
    expect(screen.getByRole('listbox')).toBeInTheDocument();
  });

  it('debounces search calls', async () => {
    mockAutocompleteSuccess([]);
    setup();
    const input = screen.getByRole('combobox');

    fireEvent.change(input, { target: { value: 'Av.' } });
    fireEvent.change(input, { target: { value: 'Av. L' } });
    fireEvent.change(input, { target: { value: 'Av. Lib' } });

    await act(() => vi.advanceTimersByTimeAsync(550));

    expect(mockGet).toHaveBeenCalledTimes(1);
  });

  it('handles null predictions from API', async () => {
    mockAutocompleteJson.mockResolvedValue({ predictions: null });
    setup();

    await typeAndFlush(screen.getByRole('combobox'), 'Av. Libertador');

    expect(screen.queryByRole('listbox')).not.toBeInTheDocument();
  });

  it('calls onClear when user edits after confirmed selection', () => {
    const { onClear } = setup({ value: 'Av. Libertador 1234, CABA', confirmed: true });
    const input = screen.getByRole('combobox');

    fireEvent.change(input, { target: { value: 'Av. Libertador' } });

    expect(onClear).toHaveBeenCalledTimes(1);
  });

  it('does not call onClear when typing without prior selection', () => {
    const { onClear } = setup({ value: '', confirmed: false });
    const input = screen.getByRole('combobox');

    fireEvent.change(input, { target: { value: 'Av.' } });

    expect(onClear).not.toHaveBeenCalled();
  });
});
