import { useState } from 'react';
import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { useMoneyInput } from './useMoneyInput';

/** A minimal controlled host, the same `value`/`onChange` shape every money field uses. */
function Harness({ onChange }: { onChange: (v: number | undefined) => void }) {
  const [value, setValue] = useState<number | undefined>(undefined);
  const { displayValue, inputRef, handleChange } = useMoneyInput({
    value,
    onChange: (v) => {
      setValue(v);
      onChange(v);
    },
  });

  return <input aria-label="Monto" ref={inputRef} value={displayValue} onChange={handleChange} />;
}

function renderHarness(onChange = vi.fn()) {
  render(<Harness onChange={onChange} />);
  return { input: screen.getByLabelText<HTMLInputElement>('Monto'), onChange };
}

describe('useMoneyInput — formatting as you type', () => {
  it('groups thousands live and reports the plain number', async () => {
    const user = userEvent.setup();
    const { input, onChange } = renderHarness();

    await user.type(input, '150000');

    expect(input).toHaveValue('150.000');
    expect(onChange).toHaveBeenLastCalledWith(150000);
  });

  it('reports undefined, never NaN, once the field is cleared', async () => {
    const user = userEvent.setup();
    const { input, onChange } = renderHarness();

    await user.type(input, '500');
    await user.clear(input);

    expect(input).toHaveValue('');
    expect(onChange).toHaveBeenLastCalledWith(undefined);
  });

  it('deleting a digit reformats the remaining amount', async () => {
    const user = userEvent.setup();
    const { input, onChange } = renderHarness();

    await user.type(input, '1500');
    expect(input).toHaveValue('1.500');

    await user.type(input, '{Backspace}');

    expect(input).toHaveValue('150');
    expect(onChange).toHaveBeenLastCalledWith(150);
  });

  it('pastes a "$ 1.500" amount as the plain number', async () => {
    const user = userEvent.setup();
    const { input, onChange } = renderHarness();

    await user.click(input);
    await user.paste('$ 1.500');

    expect(input).toHaveValue('1.500');
    expect(onChange).toHaveBeenLastCalledWith(1500);
  });

  it('drops a pasted decimal comma instead of inflating the amount', async () => {
    const user = userEvent.setup();
    const { input, onChange } = renderHarness();

    await user.click(input);
    await user.paste('1.500,50');

    expect(input).toHaveValue('1.500');
    expect(onChange).toHaveBeenLastCalledWith(1500);
  });

  it('caps the amount at the max digit length instead of growing without bound', async () => {
    const user = userEvent.setup();
    const { input, onChange } = renderHarness();

    await user.click(input);
    await user.paste('9'.repeat(20));

    expect(onChange).toHaveBeenLastCalledWith(999999999999);
  });

  it('keeps the caret after the same digit it followed before an edit in the middle', async () => {
    const user = userEvent.setup();
    const { input } = renderHarness();

    await user.type(input, '150000');
    expect(input).toHaveValue('150.000');

    // Caret between the two leading digits ("1|50.000"), then insert a "9":
    // the digit count before the caret (1) must land after the same single
    // digit in the reformatted string, not at the end of it.
    // `initialSelectionStart`/`End` simulate a click that places the caret
    // there before typing — a plain `input.setSelectionRange` would be
    // clobbered by userEvent re-focusing the element on the next `type` call.
    await user.type(input, '9', { initialSelectionStart: 1, initialSelectionEnd: 1 });

    expect(input).toHaveValue('1.950.000');
    // Right after the "9" that was just typed ("1.9|50.000") — the digit
    // count before the caret (2: "1" and "9") landed after the same two
    // digits in the reformatted string.
    expect(input.selectionStart).toBe(3);
  });
});
