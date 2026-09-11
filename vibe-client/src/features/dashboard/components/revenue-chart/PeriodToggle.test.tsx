import { render, screen, fireEvent } from '@testing-library/react';
import { PeriodToggle } from './PeriodToggle';

describe('PeriodToggle', () => {
  it('calls onChange with the clicked period', () => {
    const onChange = vi.fn();
    render(<PeriodToggle period="week" onChange={onChange} />);
    fireEvent.click(screen.getByRole('button', { name: 'Mes' }));
    expect(onChange).toHaveBeenCalledWith('month');
  });

  // `role="tab"` without arrow-key navigation or an associated `tabpanel` is
  // ARIA misapplied (the tablist pattern has a contract this toggle never
  // implemented) — plain buttons with aria-pressed describe this
  // mutually-exclusive toggle correctly instead.
  it('marks the active period with aria-pressed on a plain button, not a tab role', () => {
    render(<PeriodToggle period="week" onChange={vi.fn()} />);
    const weekButton = screen.getByRole('button', { name: 'Semana' });
    const monthButton = screen.getByRole('button', { name: 'Mes' });
    expect(weekButton).toHaveAttribute('aria-pressed', 'true');
    expect(monthButton).toHaveAttribute('aria-pressed', 'false');
    expect(screen.queryByRole('tab')).not.toBeInTheDocument();
    expect(screen.queryByRole('tablist')).not.toBeInTheDocument();
  });
});
