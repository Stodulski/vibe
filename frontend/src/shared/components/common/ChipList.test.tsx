import { describe, it, expect, vi } from 'vitest';
import { screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { renderWithProviders } from '@/test/test-utils';
import { Chip, ChipList } from './ChipList';

describe('ChipList', () => {
  it('keeps a plain click on a chip working now that the row can be dragged', async () => {
    const onClick = vi.fn();
    renderWithProviders(
      <ChipList aria-label="Categorías">
        <Chip selected={false} onClick={onClick}>
          Bebidas
        </Chip>
      </ChipList>,
    );

    await userEvent.click(screen.getByRole('button', { name: 'Bebidas' }));

    expect(onClick).toHaveBeenCalledTimes(1);
    expect(screen.getByRole('group', { name: 'Categorías' })).toBeInTheDocument();
  });
});
