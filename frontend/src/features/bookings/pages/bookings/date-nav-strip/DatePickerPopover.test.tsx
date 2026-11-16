import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { DatePickerPopover } from './DatePickerPopover';

describe('DatePickerPopover', () => {
  it('shows the short date below sm and the long one from sm, never truncating either', () => {
    render(
      <DatePickerPopover
        selectedDate="2026-12-07"
        dateLabel="lunes 7 de diciembre"
        isToday={false}
        calendarOpen={false}
        onCalendarOpenChange={() => undefined}
        onDateSelect={() => undefined}
      />,
    );

    const short = screen.getByText('lun 7 dic');
    const long = screen.getByText('lunes 7 de diciembre');
    expect(short).toHaveClass('sm:hidden');
    expect(long).toHaveClass('hidden', 'sm:inline');
    expect(short.className).not.toMatch(/truncate/);
    expect(screen.getByRole('button', { name: 'Fecha: lunes 7 de diciembre' })).toBeInTheDocument();
  });
});
