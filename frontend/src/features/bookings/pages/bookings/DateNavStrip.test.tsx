import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { DateNavStrip } from './DateNavStrip';

vi.mock('@/shared/components/ui/button', async () => {
  const { passthrough } = await import('@/test/ui-mocks');
  return { Button: passthrough('button') };
});
vi.mock('@/shared/components/ui/popover', async () => {
  const { passthrough } = await import('@/test/ui-mocks');
  return {
    Popover: passthrough('div'),
    PopoverContent: passthrough('div'),
    PopoverTrigger: passthrough('div'),
  };
});
vi.mock('@/shared/components/ui/calendar', () => ({
  Calendar: () => <div data-testid="calendar">Calendar</div>,
}));

const baseProps = {
  selectedDate: '2026-03-18',
  dateLabel: '18 de marzo de 2026',
  isToday: true,
  calendarOpen: false,
  onCalendarOpenChange: vi.fn(),
  onDateSelect: vi.fn(),
  onPrevDay: vi.fn(),
  onNextDay: vi.fn(),
  onGoToToday: vi.fn(),
};

describe('DateNavStrip', () => {
  it('renders date label', () => {
    render(<DateNavStrip {...baseProps} />);
    expect(screen.getByText('18 de marzo de 2026')).toBeInTheDocument();
  });

  it('renders previous day button', () => {
    render(<DateNavStrip {...baseProps} />);
    expect(screen.getByLabelText('Día anterior')).toBeInTheDocument();
  });

  it('renders next day button', () => {
    render(<DateNavStrip {...baseProps} />);
    expect(screen.getByLabelText('Día siguiente')).toBeInTheDocument();
  });

  it('calls onPrevDay when clicking previous', () => {
    render(<DateNavStrip {...baseProps} />);
    fireEvent.click(screen.getByLabelText('Día anterior'));
    expect(baseProps.onPrevDay).toHaveBeenCalled();
  });

  it('calls onNextDay when clicking next', () => {
    render(<DateNavStrip {...baseProps} />);
    fireEvent.click(screen.getByLabelText('Día siguiente'));
    expect(baseProps.onNextDay).toHaveBeenCalled();
  });

  it('shows today badge when isToday', () => {
    render(<DateNavStrip {...baseProps} />);
    expect(screen.getByText('Hoy')).toBeInTheDocument();
  });

  it('shows go to today button when not today', () => {
    render(<DateNavStrip {...baseProps} isToday={false} />);
    expect(screen.getByText('Ir a hoy')).toBeInTheDocument();
  });
});
