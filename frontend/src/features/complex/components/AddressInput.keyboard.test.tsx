import { screen, fireEvent, waitFor } from '@testing-library/react';
import { vi } from 'vitest';
import { mockAutocompleteSuccess, typeAndFlush, setup } from './address-input-test-helpers';

describe('AddressInput keyboard navigation', () => {
  beforeEach(() => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    vi.clearAllMocks();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('navigates with arrow keys and selects with Enter', async () => {
    mockAutocompleteSuccess();
    const { onChange } = setup();
    const input = screen.getByRole('combobox');

    await typeAndFlush(input, 'Av. Libertador');

    await waitFor(() => {
      expect(screen.getByRole('listbox')).toBeInTheDocument();
    });

    fireEvent.keyDown(input, { key: 'ArrowDown' });
    expect(screen.getAllByRole('option')[0]).toHaveAttribute('aria-selected', 'true');

    fireEvent.keyDown(input, { key: 'ArrowDown' });
    expect(screen.getAllByRole('option')[1]).toHaveAttribute('aria-selected', 'true');

    fireEvent.keyDown(input, { key: 'Enter' });
    expect(onChange).toHaveBeenCalledWith('Av. Libertador 5678, Vicente Lopez, Buenos Aires, Argentina');
  });

  it('closes dropdown on Escape', async () => {
    mockAutocompleteSuccess();
    setup();
    const input = screen.getByRole('combobox');

    await typeAndFlush(input, 'Av. Libertador');

    await waitFor(() => {
      expect(screen.getByRole('listbox')).toBeInTheDocument();
    });

    fireEvent.keyDown(input, { key: 'Escape' });
    expect(screen.queryByRole('listbox')).not.toBeInTheDocument();
  });

  it('wraps around when navigating past last option', async () => {
    mockAutocompleteSuccess();
    setup();
    const input = screen.getByRole('combobox');

    await typeAndFlush(input, 'Av. Libertador');

    await waitFor(() => {
      expect(screen.getByRole('listbox')).toBeInTheDocument();
    });

    fireEvent.keyDown(input, { key: 'ArrowDown' }); // 0
    fireEvent.keyDown(input, { key: 'ArrowDown' }); // 1
    fireEvent.keyDown(input, { key: 'ArrowDown' }); // wraps to 0
    expect(screen.getAllByRole('option')[0]).toHaveAttribute('aria-selected', 'true');

    fireEvent.keyDown(input, { key: 'ArrowUp' }); // wraps to 1
    expect(screen.getAllByRole('option')[1]).toHaveAttribute('aria-selected', 'true');
  });
});
