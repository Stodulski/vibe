import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { DatePickerPopover } from './DatePickerPopover';

describe('DatePickerPopover', () => {
  it('shows both the short and the long date, and names the trigger', () => {
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

    expect(screen.getByText('lun 7 dic')).toBeInTheDocument();
    expect(screen.getByText('lunes 7 de diciembre')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Fecha: lunes 7 de diciembre' })).toBeInTheDocument();
  });
});
