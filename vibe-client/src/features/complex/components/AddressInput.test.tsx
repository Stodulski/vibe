import { screen, waitFor } from '@testing-library/react';
import { vi } from 'vitest';
import {
  mockAutocompleteSuccess,
  mockAutocompleteJson,
  mockGet,
  typeAndFlush,
  setup,
} from './address-input-test-helpers';

describe('AddressInput', () => {
  beforeEach(() => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    vi.clearAllMocks();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('renders input with placeholder', () => {
    setup();
    expect(screen.getByPlaceholderText('Ej: Av. Libertador 1234, Palermo, CABA')).toBeInTheDocument();
  });

  it('has combobox role and aria attributes', () => {
    setup();
    const input = screen.getByRole('combobox');
    expect(input).toHaveAttribute('aria-expanded', 'false');
    expect(input).toHaveAttribute('aria-autocomplete', 'list');
    expect(input).toHaveAttribute('autocomplete', 'off');
  });

  it('does not search for queries shorter than 3 characters', async () => {
    setup();
    await typeAndFlush(screen.getByRole('combobox'), 'Av');
    expect(mockGet).not.toHaveBeenCalled();
  });

  it('shows predictions after typing 3+ characters', async () => {
    mockAutocompleteSuccess();
    setup();

    await typeAndFlush(screen.getByRole('combobox'), 'Av. Libertador');

    await waitFor(() => {
      expect(screen.getByRole('listbox')).toBeInTheDocument();
    });

    const options = screen.getAllByRole('option');
    expect(options).toHaveLength(2);
    expect(options[0]).toHaveTextContent('Av. Libertador 1234');
    expect(options[0]).toHaveTextContent('Palermo, CABA, Argentina');
  });

  it('calls places/autocomplete with input and session_token', async () => {
    mockAutocompleteSuccess([]);
    setup();

    await typeAndFlush(screen.getByRole('combobox'), 'Av. Lib');

    await waitFor(() => {
      expect(mockGet).toHaveBeenCalledWith(
        'places/autocomplete',
        expect.objectContaining({
          searchParams: expect.objectContaining({ input: 'Av. Lib' }) as unknown as Record<string, string>,
        }),
      );
    });

    const params = mockGet.mock.calls[0]?.[1]?.searchParams;
    expect(params?.session_token).toBeDefined();
  });

  it('handles autocomplete API failure gracefully', async () => {
    mockAutocompleteJson.mockRejectedValue(new Error('Network error'));
    setup();

    await typeAndFlush(screen.getByRole('combobox'), 'Av. Libertador');

    expect(screen.queryByRole('listbox')).not.toBeInTheDocument();
  });
});
