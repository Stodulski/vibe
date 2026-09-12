import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { DateSelector } from './DateSelector';
import type { Schedule } from '@/shared/types/api.types';

// Mock scrollIntoView for the test DOM
Element.prototype.scrollIntoView = vi.fn();

const mockSchedules: Schedule[] = [
  {
    id: 's1',
    complex_id: 'c1',
    day: 'monday',
    open_time: '08:00',
    close_time: '23:00',
    is_closed: false,
  },
  {
    id: 's2',
    complex_id: 'c1',
    day: 'tuesday',
    open_time: '08:00',
    close_time: '23:00',
    is_closed: false,
  },
  {
    id: 's3',
    complex_id: 'c1',
    day: 'wednesday',
    open_time: '08:00',
    close_time: '23:00',
    is_closed: false,
  },
  {
    id: 's4',
    complex_id: 'c1',
    day: 'thursday',
    open_time: '08:00',
    close_time: '23:00',
    is_closed: false,
  },
  {
    id: 's5',
    complex_id: 'c1',
    day: 'friday',
    open_time: '08:00',
    close_time: '23:00',
    is_closed: false,
  },
  {
    id: 's6',
    complex_id: 'c1',
    day: 'saturday',
    open_time: '09:00',
    close_time: '22:00',
    is_closed: false,
  },
  {
    id: 's7',
    complex_id: 'c1',
    day: 'sunday',
    open_time: '00:00',
    close_time: '00:00',
    is_closed: true,
  },
];

describe('DateSelector', () => {
  const today = new Date();

  const defaultProps = {
    selectedDate: today,
    onDateSelect: vi.fn(),
    schedules: mockSchedules,
  };

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders the selected date as heading', () => {
    render(<DateSelector {...defaultProps} />);
    expect(screen.getByRole('heading', { level: 2 })).toBeInTheDocument();
  });

  it('renders 14 date options', () => {
    render(<DateSelector {...defaultProps} />);
    const buttons = screen.getAllByRole('radio');
    expect(buttons.length).toBe(14);
  });

  it('shows "Hoy" label for today', () => {
    render(<DateSelector {...defaultProps} />);
    expect(screen.getByText(/hoy/i)).toBeInTheDocument();
  });

  it('disables buttons for closed days', () => {
    render(<DateSelector {...defaultProps} />);
    const buttons = screen.getAllByRole('radio');
    const disabledButtons = buttons.filter((btn) => btn.hasAttribute('disabled'));
    // At least some Sundays should be disabled in the 14-day range
    expect(disabledButtons.length).toBeGreaterThanOrEqual(1);
  });

  it('shows "Cerrado" text on closed days', () => {
    render(<DateSelector {...defaultProps} />);
    const closedLabels = screen.queryAllByText(/cerrado/i);
    // Sundays in the 14-day range should show "Cerrado"
    expect(closedLabels.length).toBeGreaterThanOrEqual(1);
  });

  it('calls onDateSelect when an available day is clicked', async () => {
    const user = userEvent.setup();
    render(<DateSelector {...defaultProps} />);
    const buttons = screen.getAllByRole('radio');
    // Find first enabled button
    const enabledButton = buttons.find((btn) => !btn.hasAttribute('disabled'));
    if (enabledButton) {
      await user.click(enabledButton);
      expect(defaultProps.onDateSelect).toHaveBeenCalled();
    }
  });

  it('does not call onDateSelect when a closed day is clicked', async () => {
    const user = userEvent.setup();
    render(<DateSelector {...defaultProps} />);
    const buttons = screen.getAllByRole('radio');
    const disabledButton = buttons.find((btn) => btn.hasAttribute('disabled'));
    if (disabledButton) {
      await user.click(disabledButton);
      expect(defaultProps.onDateSelect).not.toHaveBeenCalled();
    }
  });
});

// Roving tabindex + arrow-key navigation (WCAG 2.1.1 keyboard trap fix): the
// date strip used to put every loaded date in the Tab order, so Tab never
// reached the duration/hour controls below it.
describe('DateSelector keyboard navigation and ARIA', () => {
  const today = new Date();

  const defaultProps = {
    selectedDate: today,
    onDateSelect: vi.fn(),
    schedules: mockSchedules,
  };

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('has an accessible group label ("Fecha")', () => {
    render(<DateSelector {...defaultProps} />);
    expect(screen.getByRole('radiogroup', { name: 'Fecha' })).toBeInTheDocument();
  });

  it('only the selected date is in the tab order (roving tabindex)', () => {
    render(<DateSelector {...defaultProps} />);
    const options = screen.getAllByRole('radio');
    const tabbable = options.filter((btn) => btn.tabIndex === 0);
    expect(tabbable).toHaveLength(1);
    expect(tabbable[0]).toHaveAttribute('aria-checked', 'true');
  });

  it('marks the selected date with aria-checked="true" and others "false"', () => {
    render(<DateSelector {...defaultProps} />);
    const options = screen.getAllByRole('radio');
    const checked = options.filter((btn) => btn.getAttribute('aria-checked') === 'true');
    expect(checked).toHaveLength(1);
  });

  it('Tab moves from the selected date straight to the next control (one press)', async () => {
    const user = userEvent.setup();
    render(
      <div>
        <DateSelector {...defaultProps} />
        <button type="button">Next control</button>
      </div>,
    );
    const options = screen.getAllByRole('radio');
    const selected = options.find((btn) => btn.getAttribute('aria-checked') === 'true');
    selected?.focus();
    expect(selected).toHaveFocus();

    await user.tab();

    expect(screen.getByRole('button', { name: 'Next control' })).toHaveFocus();
  });

  it('ArrowRight moves the selection to the next enabled date', async () => {
    const user = userEvent.setup();
    const onDateSelect = vi.fn();
    render(<DateSelector {...defaultProps} onDateSelect={onDateSelect} />);
    const options = screen.getAllByRole('radio');
    const selected = options.find((btn) => btn.getAttribute('aria-checked') === 'true');
    selected?.focus();

    await user.keyboard('{ArrowRight}');

    expect(onDateSelect).toHaveBeenCalledTimes(1);
  });
});
