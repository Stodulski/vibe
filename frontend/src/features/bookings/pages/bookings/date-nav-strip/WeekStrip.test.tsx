import { render, screen } from '@testing-library/react';
import { WeekStrip } from './WeekStrip';

describe('WeekStrip', () => {
  it('marks the selected day with aria-pressed, not color alone', () => {
    render(<WeekStrip selectedDate="2026-09-10" onDateSelect={vi.fn()} />);
    const selected = screen.getByText('10').closest('button');
    expect(selected).toHaveAttribute('aria-pressed', 'true');

    const others = screen.getAllByRole('button').filter((b) => b !== selected);
    expect(others.length).toBeGreaterThan(0);
    for (const day of others) {
      expect(day).toHaveAttribute('aria-pressed', 'false');
    }
  });
});
