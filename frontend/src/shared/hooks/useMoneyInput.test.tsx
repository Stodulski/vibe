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
    // Right after the "9" that was just typed ("1.9|50.000") - the digit
    // count before the caret (2: "1" and "9") landed after the same two
    // digits in the reformatted string.
    expect(input.selectionStart).toBe(3);
  });
});

describe('useMoneyInput - a decimal comma is a real, accepted amount, never silently dropped or corrupted', () => {
  // Regression coverage for a real bug fixed in PR #135: typing "1.500,50"
  // one keystroke at a time used to reformat away the comma the instant it
  // landed, so the following "5" and "0" silently read as two more
  // thousands digits of the INTEGER part - "1.500,50" ended up 150050 pesos,
  // a 100x amount, with no error anywhere. Centavos are now a real, accepted
  // amount (not an error to reject), but the same 100x/10x corruption must
  // still never happen at any point while typing.

  it('typing a decimal comma one keystroke at a time never inflates the amount', async () => {
    const user = userEvent.setup();
    const { input, onChange } = renderHarness();

    await user.type(input, '1500,50');

    // The integer part was already grouped ("1.500") by the time the comma
    // landed. What matters is what does NOT happen: it is not reformatted
    // into "1.500" alone (losing the ",50"), and it is NOT the corrupted
    // "150.050"/150050 the original bug produced.
    expect(input).toHaveValue('1.500,50');
    expect(onChange).toHaveBeenLastCalledWith(1500.5);
    expect(onChange).not.toHaveBeenCalledWith(150050);
  });

  it('typing "1500,00" key by key never reports the 10x amount at any point', async () => {
    const user = userEvent.setup();
    const { input, onChange } = renderHarness();

    await user.type(input, '1500,00');

    expect(input).toHaveValue('1.500,00');
    expect(onChange).toHaveBeenLastCalledWith(1500);
    expect(onChange).not.toHaveBeenCalledWith(15000);
  });

  it('pastes a real decimal amount, shown grouped and reported as the exact fraction', async () => {
    const user = userEvent.setup();
    const { input, onChange } = renderHarness();

    await user.click(input);
    await user.paste('1.500,50');

    expect(input).toHaveValue('1.500,50');
    expect(onChange).toHaveBeenLastCalledWith(1500.5);
  });

  it('pastes "$ 1.500,50" as 1500.5, shown grouped', async () => {
    const user = userEvent.setup();
    const { input, onChange } = renderHarness();

    await user.click(input);
    await user.paste('$ 1.500,50');

    expect(input).toHaveValue('1.500,50');
    expect(onChange).toHaveBeenLastCalledWith(1500.5);
  });

  // Regression coverage for the money-centavos change: a 3rd decimal digit
  // is never appended anywhere, typed or pasted.
  it('drops a 3rd decimal digit typed past the cap, without changing the reported amount', async () => {
    const user = userEvent.setup();
    const { input, onChange } = renderHarness();

    await user.type(input, '1500,50');
    expect(input).toHaveValue('1.500,50');

    await user.type(input, '5');

    expect(input).toHaveValue('1.500,50');
    expect(onChange).toHaveBeenLastCalledWith(1500.5);
  });

  it('pasting "1.500,505" drops the 3rd decimal digit and reports 1500.5', async () => {
    const user = userEvent.setup();
    const { input, onChange } = renderHarness();

    await user.click(input);
    await user.paste('1.500,505');

    expect(input).toHaveValue('1.500,50');
    expect(onChange).toHaveBeenLastCalledWith(1500.5);
  });
});

describe('useMoneyInput - a zero decimal ("," / ",0" / ",00") is never collapsed mid-typing either', () => {
  // Regression coverage, one digit later than the block above: ",0"/",00" is
  // unambiguous ("no centavos") once the person is DONE typing, but
  // collapsing it the instant it lands is the same 100x bug one digit
  // further along - the comma disappears from the DOM, and the next typed
  // digit is then read as another thousands digit instead of a centavos
  // digit. "1500,00" typed one keystroke at a time must never become 15000
  // mid-typing (or 150000 counted in cents downstream).

  it('pasting an explicit ",00" (no centavos) stays visible until blur, then collapses', async () => {
    const user = userEvent.setup();
    const { input, onChange } = renderHarness();

    await user.click(input);
    await user.paste('$ 1.500,00');

    expect(input).toHaveValue('1.500,00');
    expect(onChange).toHaveBeenLastCalledWith(1500);

    await user.tab();

    expect(input).toHaveValue('1.500');
  });

  it('typing ",00" one keystroke at a time never inflates the amount, even mid-typing', async () => {
    const user = userEvent.setup();
    const { input, onChange } = renderHarness();

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

  it('a non-zero digit after a zero decimal is read as the real centavos amount', async () => {
    const user = userEvent.setup();
    const { input, onChange } = renderHarness();

    await user.type(input, '1500,0');
    expect(input).toHaveValue('1.500,0');
    expect(onChange).toHaveBeenLastCalledWith(1500);

    await user.type(input, '5');

    expect(input).toHaveValue('1.500,05');
    expect(onChange).toHaveBeenLastCalledWith(1500.05);
  });

  // Replaces the old "3 keystrokes after a trailing comma" case, which used
  // to accept an unbounded number of decimal digits ("1500,007" -> 1500.007).
  // The money-centavos change caps decimals at 2: a 3rd digit typed after two
  // zeros is dropped entirely, not appended as a 3rd decimal place.
  it('a 3rd digit typed after two zero decimals is dropped, staying at ",00"', async () => {
    const user = userEvent.setup();
    const { input, onChange } = renderHarness();

    await user.type(input, '1500,');
    await user.type(input, '0');
    await user.type(input, '0');
    await user.type(input, '7');

    expect(input).toHaveValue('1.500,00');
    expect(onChange).toHaveBeenLastCalledWith(1500);
    expect(onChange).not.toHaveBeenCalledWith(1500.007);
  });
});

describe('useMoneyInput - a lone trailing comma ("pending") is never collapsed mid-typing', () => {
  it('a lone trailing comma is left visible while typing, and reports the integer part', async () => {
    const user = userEvent.setup();
    const { input, onChange } = renderHarness();

    await user.type(input, '1500,');

    // The comma itself is NOT dropped - the integer part was already grouped
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

  // Replaces the old "stays visible, shows a centavos error" expectation: a
  // decimal amount is now accepted, and blur pads it to its canonical 2
  // decimal digits rather than leaving it at whatever the person happened to
  // type ("hasta 2 decimales" behavior, see `formatMoneyValue`).
  it('a single decimal digit is padded to 2 on blur, the amount itself unchanged', async () => {
    const user = userEvent.setup();
    const { input, onChange } = renderHarness();

    await user.type(input, '1500,5');
    expect(onChange).toHaveBeenLastCalledWith(1500.5);

    await user.tab();

    expect(input).toHaveValue('1.500,50');
    expect(onChange).toHaveBeenLastCalledWith(1500.5);
  });

  it('an already-2-digit decimal is unchanged on blur', async () => {
    const user = userEvent.setup();
    const { input, onChange } = renderHarness();

    await user.type(input, '1500,05');
    await user.tab();

    expect(input).toHaveValue('1.500,05');
    expect(onChange).toHaveBeenLastCalledWith(1500.05);
  });
});
