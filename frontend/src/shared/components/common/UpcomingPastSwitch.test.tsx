import { describe, it, expect, vi } from 'vitest';
import { renderWithProviders as render, screen, userEvent } from '@/test/test-utils';
import { UpcomingPastSwitch } from './UpcomingPastSwitch';

describe('UpcomingPastSwitch', () => {
  it('shows Próximos and Pasados, marking the active one', () => {
    render(<UpcomingPastSwitch value="past" onChange={vi.fn()} />);
    expect(screen.getByRole('button', { name: 'Pasados' })).toHaveAttribute('aria-pressed', 'true');
    expect(screen.getByRole('button', { name: 'Próximos' })).toHaveAttribute('aria-pressed', 'false');
  });

  it('reports the pressed side', async () => {
    const onChange = vi.fn();
    render(<UpcomingPastSwitch value="upcoming" onChange={onChange} />);
    await userEvent.setup().click(screen.getByRole('button', { name: 'Pasados' }));
    expect(onChange).toHaveBeenCalledWith('past');
  });

  it('keeps each button at least 40px tall', () => {
    render(<UpcomingPastSwitch value="upcoming" onChange={vi.fn()} />);
    for (const name of ['Próximos', 'Pasados']) {
      expect(screen.getByRole('button', { name })).toHaveClass('h-10');
    }
  });
});
