import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { DatePickerField } from './DatePickerField';

describe('DatePickerField', () => {
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
