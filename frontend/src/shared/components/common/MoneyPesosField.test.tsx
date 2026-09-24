import { useState } from 'react';
import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MoneyPesosField } from './MoneyPesosField';

/** `MoneyPesosField` is fully controlled — mirrors how every real caller feeds `onChange` back into `value`. */
function ControlledHarness({ onChange }: { onChange: (v: number | undefined) => void }) {
  const [value, setValue] = useState<number | undefined>(undefined);
  return (
    <MoneyPesosField
      id="amount"
      label="Monto"
      value={value}
      onChange={(v) => {
        setValue(v);
        onChange(v);
      }}
    />
  );
}

describe('MoneyPesosField — formats as you type', () => {
  it('shows "150.000" and reports 150000 once the amount is typed', async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(<ControlledHarness onChange={onChange} />);

    await user.type(screen.getByLabelText('Monto'), '150000');

    expect(screen.getByLabelText('Monto')).toHaveValue('150.000');
    expect(onChange).toHaveBeenLastCalledWith(150000);
  });

  it('is a text input, not a number spinner', () => {
    render(<MoneyPesosField id="amount" label="Monto" value={undefined} onChange={vi.fn()} />);

    expect(screen.getByRole('textbox', { name: 'Monto' })).toBeInTheDocument();
  });

  it('renders an untouched or NaN value as an empty field, not "undefined"/"NaN"', () => {
    const { rerender } = render(<MoneyPesosField id="amount" label="Monto" value={undefined} onChange={vi.fn()} />);
    expect(screen.getByLabelText('Monto')).toHaveValue('');

    rerender(<MoneyPesosField id="amount" label="Monto" value={Number.NaN} onChange={vi.fn()} />);
    expect(screen.getByLabelText('Monto')).toHaveValue('');
  });

  it('renders an existing value already grouped', () => {
    render(<MoneyPesosField id="amount" label="Monto" value={1500} onChange={vi.fn()} />);
    expect(screen.getByLabelText('Monto')).toHaveValue('1.500');
  });
});
