import { useState } from 'react';
import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { useMoneyInput } from './useMoneyInput';

/** A minimal controlled host, the same `value`/`onChange` shape every money field uses. */
function Harness({ onChange }: { onChange: (v: number | undefined) => void }) {
  const [value, setValue] = useState<number | undefined>(undefined);
  const { displayValue, inputRef, handleChange, handleBlur } = useMoneyInput({
    value,
    onChange: (v) => {
      setValue(v);
      onChange(v);
    },
  });

  return <input aria-label="Monto" ref={inputRef} value={displayValue} onChange={handleChange} onBlur={handleBlur} />;
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

describe('useMoneyInput — a decimal comma is never silently dropped', () => {
  // Regression coverage for a real bug: typing "1.500,50" one keystroke at a
  // time used to reformat away the comma the instant it landed, so the
  // following "5" and "0" silently read as two more thousands digits of the
  // INTEGER part — "1.500,50" ended up 150050 pesos, a 100x amount, with no
  // error anywhere. `analyzeMoneyInput`'s 'invalid' case is what this whole
  // describe block is pinning down.

  it('typing a decimal comma one keystroke at a time never inflates the amount', async () => {
    const user = userEvent.setup();
    const { input, onChange } = renderHarness();

    await user.type(input, '1500,50');

    // The integer part was already grouped ("1.500") by the time the comma
    // landed — that grouping is harmless and stays. What matters is what
    // does NOT happen: it is not reformatted into "1.500" alone (losing the
    // ",50"), and it is NOT the corrupted "150.050" the original bug produced.
    expect(input).toHaveValue('1.500,50');
    expect(onChange).toHaveBeenLastCalledWith(1500.5);
    expect(onChange).not.toHaveBeenCalledWith(150050);
  });

  it('pasting a real decimal amount is shown verbatim and reported as a fraction, never rounded away', async () => {
    const user = userEvent.setup();
    const { input, onChange } = renderHarness();

    await user.click(input);
    await user.paste('1.500,50');

    expect(input).toHaveValue('1.500,50');
    expect(onChange).toHaveBeenLastCalledWith(1500.5);
  });
});

describe('useMoneyInput — a zero decimal ("," / ",0" / ",00") is never collapsed mid-typing either', () => {
  // Regression coverage, one digit later than the block above: ",0"/",00" is
  // unambiguous ("no centavos") once the person is DONE typing, but
  // collapsing it the instant it lands is the same 100x bug one digit
  // further along — the comma disappears from the DOM, and the next typed
  // digit is then read as another thousands digit instead of a centavos
  // digit. "1500,00" typed one keystroke at a time must never become 15000
  // mid-typing (or 150000 counted in cents downstream).

  it('pasting an explicit ",00" (no centavos) stays visible until blur, then collapses', async () => {
    const user = userEvent.setup();
    const { input, onChange } = renderHarness();

    await user.click(input);
    await user.paste('$ 1.500,00');

    // NOT collapsed immediately, and shown exactly as pasted (same as the
    // 'invalid' case) — see the "typing ',00' one keystroke at a time" test
    // below for why an immediate collapse here is the same 100x bug, one
    // digit later than the lone-trailing-comma case.
    expect(input).toHaveValue('$ 1.500,00');
    expect(onChange).toHaveBeenLastCalledWith(1500);

    await user.tab();

    expect(input).toHaveValue('1.500');
  });

  it('typing ",00" one keystroke at a time never inflates the amount, even mid-typing', async () => {
    const user = userEvent.setup();
    const { input, onChange } = renderHarness();

    // Regression coverage, one digit later than the lone-comma case above: a
    // ",0"/",00" decimal is unambiguous ("no centavos") once the person is
    // DONE typing, but collapsing it the instant the second "0" lands still
    // drops the comma from the DOM — a still-in-progress "1500,00" typed one
    // keystroke at a time must never become 15000 at any point along the way.
    await user.type(input, '1500,');
    expect(onChange).not.toHaveBeenCalledWith(15000);

    await user.type(input, '0');
    expect(input).toHaveValue('1.500,0');
    expect(onChange).toHaveBeenLastCalledWith(1500);
    expect(onChange).not.toHaveBeenCalledWith(15000);

    await user.type(input, '0');
    expect(input).toHaveValue('1.500,00');
    expect(onChange).toHaveBeenLastCalledWith(1500);
    expect(onChange).not.toHaveBeenCalledWith(15000);

    await user.tab();

    expect(input).toHaveValue('1.500');
    expect(onChange).toHaveBeenLastCalledWith(1500);
    expect(onChange).not.toHaveBeenCalledWith(15000);
  });

  it('a non-zero digit after a zero decimal turns it invalid, shown verbatim', async () => {
    const user = userEvent.setup();
    const { input, onChange } = renderHarness();

    await user.type(input, '1500,0');
    expect(input).toHaveValue('1.500,0');
    expect(onChange).toHaveBeenLastCalledWith(1500);

    await user.type(input, '5');

    expect(input).toHaveValue('1.500,05');
    expect(onChange).toHaveBeenLastCalledWith(1500.05);
  });

  it('a non-zero digit landing three keystrokes after a trailing comma is still caught', async () => {
    const user = userEvent.setup();
    const { input, onChange } = renderHarness();

    await user.type(input, '1500,');
    await user.type(input, '0');
    await user.type(input, '0');
    await user.type(input, '7');

    expect(input).toHaveValue('1.500,007');
    expect(onChange).toHaveBeenLastCalledWith(1500.007);
  });
});

describe('useMoneyInput — a lone trailing comma ("pending") is never collapsed mid-typing', () => {
  it('a lone trailing comma is left visible while typing, and reports the integer part', async () => {
    const user = userEvent.setup();
    const { input, onChange } = renderHarness();

    await user.type(input, '1500,');

    // The comma itself is NOT dropped — the integer part was already grouped
    // ("1.500") before it landed, but the trailing "," stays visible: see
    // this hook's own doc comment for why collapsing it away now is exactly
    // the bug above, one keystroke earlier.
    expect(input).toHaveValue('1.500,');
    expect(onChange).toHaveBeenLastCalledWith(1500);
  });

  it('a digit typed right after a trailing comma is read as a decimal, never appended to the integer part', async () => {
    const user = userEvent.setup();
    const { input, onChange } = renderHarness();

    await user.type(input, '1500,');
    await user.type(input, '5');

    expect(input).toHaveValue('1.500,5');
    expect(onChange).toHaveBeenLastCalledWith(1500.5);
    expect(onChange).not.toHaveBeenCalledWith(15005);
  });

  it('a pending trailing comma resolves to the grouped integer on blur', async () => {
    const user = userEvent.setup();
    const { input, onChange } = renderHarness();

    await user.type(input, '1500,');
    expect(input).toHaveValue('1.500,');

    await user.tab();

    expect(input).toHaveValue('1.500');
    expect(onChange).toHaveBeenLastCalledWith(1500);
  });

  it('leaves an invalid fractional amount visible on blur, so its error stays visible', async () => {
    const user = userEvent.setup();
    const { input } = renderHarness();

    await user.type(input, '1500,5');
    await user.tab();

    expect(input).toHaveValue('1.500,5');
  });
});
