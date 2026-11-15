import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { DatePickerField } from './DatePickerField';

async function openPicker() {
  const user = userEvent.setup();
  render(<DatePickerField id="d" label="Fecha" value="" placeholder="Elegí la fecha" onSelect={vi.fn()} />);
  await user.click(screen.getByRole('button', { name: 'Fecha' }));
  return document.querySelector('[data-slot=popover-content]');
}

describe('DatePickerField', () => {
  // At 320px the calendar ran past the right edge and, opened upward from a field low in a
  // dialog, past the top. The popover is held to the room Radix measures for it.
  it('limits the calendar popover to the room available on screen', async () => {
    const popover = await openPicker();
    expect(popover).toHaveClass('max-w-(--radix-popover-content-available-width)');
    expect(popover).toHaveClass('max-h-(--radix-popover-content-available-height)');
    expect(popover).toHaveClass('overflow-y-auto');
  });

  it('reports the picked day as YYYY-MM-DD and closes', async () => {
    const user = userEvent.setup();
    const onSelect = vi.fn();
    render(
      <DatePickerField
        id="d"
        label="Fecha"
        value=""
        placeholder="Elegí la fecha"
        onSelect={onSelect}
        disablePast={false}
      />,
    );
    await user.click(screen.getByRole('button', { name: 'Fecha' }));
    await user.click(screen.getAllByRole('gridcell')[10]?.querySelector('button') as HTMLElement);
    expect(onSelect).toHaveBeenCalledWith(expect.stringMatching(/^\d{4}-\d{2}-\d{2}$/));
  });
});
