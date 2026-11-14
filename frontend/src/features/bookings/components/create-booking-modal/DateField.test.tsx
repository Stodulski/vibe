import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { useState } from 'react';
import { Popover, PopoverContent, PopoverTrigger } from '@/shared/components/ui/popover';
import { DateField, DateFieldTrigger } from './DateField';

// Regression: the trigger dropped the props `PopoverTrigger asChild` passes,
// so clicking the date button never opened the calendar.
describe('DateField', () => {
  it('opens the calendar when the date button is clicked', async () => {
    function Harness() {
      const [open, setOpen] = useState(false);
      return (
        <DateField date={undefined} calendarOpen={open} setCalendarOpen={setOpen} setValue={vi.fn()} errors={{}} />
      );
    }
    render(<Harness />);

    await userEvent.click(screen.getByRole('button', { name: /fecha/i }));

    expect(await screen.findByRole('grid')).toBeInTheDocument();
  });

  it('opens a popover when used as a bare trigger, as the series end date does', async () => {
    render(
      <Popover>
        <PopoverTrigger asChild>
          <DateFieldTrigger id="series-until" date={undefined} hasError={false} />
        </PopoverTrigger>
        <PopoverContent>contenido</PopoverContent>
      </Popover>,
    );

    await userEvent.click(screen.getByRole('button', { name: /seleccionar fecha/i }));

    expect(await screen.findByText('contenido')).toBeInTheDocument();
  });
});
