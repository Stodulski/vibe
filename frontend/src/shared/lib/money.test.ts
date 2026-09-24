import { describe, it, expect } from 'vitest';
import {
  pesosToCentavos,
  centavosToPesos,
  hasAtMostTwoDecimals,
  extractMoneyDigits,
  parseMoneyDigits,
  formatMoneyDigits,
  formatMoneyValue,
  countDigits,
  caretPositionForDigitCount,
  analyzeMoneyInput,
  composeMoneyDisplay,
  fractionalMoneyValue,
  MAX_MONEY_DIGITS,
} from './money';

describe('pesosToCentavos', () => {
  it('converts a whole peso amount', () => {
    expect(pesosToCentavos(5000)).toBe(500000);
  });

  it('rounds a peso amount with cents', () => {
    expect(pesosToCentavos(10.5)).toBe(1050);
    expect(pesosToCentavos(10.005)).toBe(1001); // floating point: 1000.5 rounds to 1001
  });

  it('converts zero', () => {
    expect(pesosToCentavos(0)).toBe(0);
  });
});

describe('centavosToPesos', () => {
  it('is the inverse of pesosToCentavos for round values', () => {
    expect(centavosToPesos(pesosToCentavos(1234.56))).toBeCloseTo(1234.56);
  });

  it('converts centavos back to pesos', () => {
    expect(centavosToPesos(150000)).toBe(1500);
  });
});

describe('hasAtMostTwoDecimals', () => {
  it('accepts whole amounts', () => {
    expect(hasAtMostTwoDecimals(1500)).toBe(true);
    expect(hasAtMostTwoDecimals(0)).toBe(true);
  });

  it('accepts amounts with 1 or 2 decimal digits', () => {
    expect(hasAtMostTwoDecimals(1500.5)).toBe(true);
    expect(hasAtMostTwoDecimals(1500.55)).toBe(true);
  });

  it('accepts a value with floating-point noise from an exact centavos amount', () => {
    // 1500.1 * 100 is 150009.99999999999 in IEEE754, not 150010.
    expect(hasAtMostTwoDecimals(1500.1)).toBe(true);
  });

  it('rejects a 3rd decimal digit', () => {
    expect(hasAtMostTwoDecimals(1500.555)).toBe(false);
    expect(hasAtMostTwoDecimals(1500.001)).toBe(false);
  });
});

describe('extractMoneyDigits', () => {
  it('keeps plain typed digits', () => {
    expect(extractMoneyDigits('150000')).toBe('150000');
  });

  it('strips a "$" and spaces from a pasted amount', () => {
    expect(extractMoneyDigits('$ 1.500')).toBe('1500');
  });

  it('strips the thousands "." this same field renders', () => {
    expect(extractMoneyDigits('1.500')).toBe('1500');
  });

  it('drops everything from the first "," onward — analyzeMoneyInput handles the decimal part', () => {
    expect(extractMoneyDigits('1.500,50')).toBe('1500');
    expect(extractMoneyDigits('1500,99')).toBe('1500');
  });

  it('returns an empty string for a blank/non-numeric input', () => {
    expect(extractMoneyDigits('')).toBe('');
    expect(extractMoneyDigits('abc')).toBe('');
  });

  it('caps the result at MAX_MONEY_DIGITS digits', () => {
    const digits = '9'.repeat(MAX_MONEY_DIGITS + 5);
    expect(extractMoneyDigits(digits)).toBe('9'.repeat(MAX_MONEY_DIGITS));
  });
});

describe('parseMoneyDigits', () => {
  it('parses a digit string to a number', () => {
    expect(parseMoneyDigits('150000')).toBe(150000);
  });

  it('reports an empty digit string as undefined, never NaN or 0', () => {
    expect(parseMoneyDigits('')).toBeUndefined();
  });

  it('drops leading zeros', () => {
    expect(parseMoneyDigits('00150')).toBe(150);
  });
});

describe('formatMoneyDigits / formatMoneyValue', () => {
  it('groups thousands with the es-AR separator', () => {
    expect(formatMoneyDigits('150000')).toBe('150.000');
    expect(formatMoneyValue(150000)).toBe('150.000');
  });

  it('renders a small amount with no separator at all', () => {
    expect(formatMoneyDigits('120')).toBe('120');
  });

  it('renders undefined and NaN as an empty string, not "undefined"/"NaN"', () => {
    expect(formatMoneyValue(undefined)).toBe('');
    expect(formatMoneyValue(Number.NaN)).toBe('');
  });

  it('shows no decimal part for a whole amount, even one that used to carry a fractional float', () => {
    expect(formatMoneyValue(1500)).toBe('1.500');
  });

  it('shows exactly 2 decimal digits for an amount with centavos, never truncated', () => {
    expect(formatMoneyValue(1500.5)).toBe('1.500,50');
    expect(formatMoneyValue(1500.05)).toBe('1.500,05');
    expect(formatMoneyValue(1500.55)).toBe('1.500,55');
  });
});

describe('countDigits', () => {
  it('counts only 0-9 characters', () => {
    expect(countDigits('1.500,50')).toBe(6);
    expect(countDigits('')).toBe(0);
  });
});

describe('caretPositionForDigitCount', () => {
  it('places the caret right after the Nth digit', () => {
    expect(caretPositionForDigitCount('150.000', 1)).toBe(1);
    expect(caretPositionForDigitCount('150.000', 3)).toBe(3);
    expect(caretPositionForDigitCount('150.000', 4)).toBe(5); // past the '.'
    expect(caretPositionForDigitCount('150.000', 6)).toBe(7);
  });

  it('places the caret at the very start for zero digits before it', () => {
    expect(caretPositionForDigitCount('150.000', 0)).toBe(0);
  });

  it('clamps to the end when asked for more digits than exist', () => {
    expect(caretPositionForDigitCount('150', 10)).toBe(3);
  });

  it('skips a decimal comma the same way it skips a thousands dot', () => {
    // "1.500,50" has 6 digits total ('1','5','0','0','5','0'); the 6th (last)
    // one is the final '0', so the caret lands at the very end of the string.
    expect(caretPositionForDigitCount('1.500,50', 6)).toBe(8);
  });
});

describe('analyzeMoneyInput', () => {
  it('reports "none" for a plain digit string with no comma', () => {
    expect(analyzeMoneyInput('150000')).toEqual({
      hasComma: false,
      integerDigits: '150000',
      decimalDigits: '',
      kind: 'none',
    });
  });

  it('reports "pending" for a lone trailing comma with nothing after it yet', () => {
    expect(analyzeMoneyInput('1500,')).toEqual({
      hasComma: true,
      integerDigits: '1500',
      decimalDigits: '',
      kind: 'pending',
    });
  });

  it('reports "zero" for a comma followed only by zeros', () => {
    expect(analyzeMoneyInput('1500,0')).toEqual({
      hasComma: true,
      integerDigits: '1500',
      decimalDigits: '0',
      kind: 'zero',
    });
    expect(analyzeMoneyInput('$ 1.500,00')).toEqual({
      hasComma: true,
      integerDigits: '1500',
      decimalDigits: '00',
      kind: 'zero',
    });
  });

  it('reports "decimal" for a comma followed by a non-zero digit — a real, accepted centavos amount', () => {
    expect(analyzeMoneyInput('1500,5')).toEqual({
      hasComma: true,
      integerDigits: '1500',
      decimalDigits: '5',
      kind: 'decimal',
    });
    expect(analyzeMoneyInput('1.500,50')).toEqual({
      hasComma: true,
      integerDigits: '1500',
      decimalDigits: '50',
      kind: 'decimal',
    });
  });

  // Regression coverage for the money-centavos change: a 3rd decimal digit is
  // capped away here, before anything downstream ever sees it — never
  // appended to the reported value, never shown.
  it('caps decimalDigits at 2 — a 3rd typed/pasted decimal digit is dropped, not appended', () => {
    expect(analyzeMoneyInput('1.500,505')).toEqual({
      hasComma: true,
      integerDigits: '1500',
      decimalDigits: '50',
      kind: 'decimal',
    });
    expect(analyzeMoneyInput('1500,007')).toEqual({
      hasComma: true,
      integerDigits: '1500',
      decimalDigits: '00',
      kind: 'zero',
    });
  });
});

describe('composeMoneyDisplay', () => {
  it('groups the integer part alone when there is no comma', () => {
    expect(composeMoneyDisplay(analyzeMoneyInput('150000'))).toBe('150.000');
  });

  it('keeps the comma and decimal digits exactly as captured, with the integer part grouped', () => {
    expect(composeMoneyDisplay(analyzeMoneyInput('1500,'))).toBe('1.500,');
    expect(composeMoneyDisplay(analyzeMoneyInput('1500,0'))).toBe('1.500,0');
    expect(composeMoneyDisplay(analyzeMoneyInput('1500,5'))).toBe('1.500,5');
    expect(composeMoneyDisplay(analyzeMoneyInput('1500,50'))).toBe('1.500,50');
  });

  // A pasted "1.500,505" must never show the dropped 3rd decimal digit.
  it('never shows a dropped 3rd decimal digit', () => {
    expect(composeMoneyDisplay(analyzeMoneyInput('1.500,505'))).toBe('1.500,50');
  });
});

describe('fractionalMoneyValue', () => {
  it('builds the real fractional pesos amount, never a corrupted integer', () => {
    expect(fractionalMoneyValue(analyzeMoneyInput('1500,5'))).toBe(1500.5);
    expect(fractionalMoneyValue(analyzeMoneyInput('1.500,50'))).toBe(1500.5);
  });

  it('defaults a missing integer part to 0', () => {
    expect(fractionalMoneyValue(analyzeMoneyInput(',5'))).toBe(0.5);
  });

  it('reports the plain integer for a pending or all-zero decimal — never 10x/100x from the comma', () => {
    expect(fractionalMoneyValue(analyzeMoneyInput('1500,'))).toBe(1500);
    expect(fractionalMoneyValue(analyzeMoneyInput('1500,0'))).toBe(1500);
    expect(fractionalMoneyValue(analyzeMoneyInput('1500,00'))).toBe(1500);
  });

  it('reports undefined for an empty field with only a stray comma', () => {
    expect(fractionalMoneyValue(analyzeMoneyInput(','))).toBeUndefined();
  });

  // The exact regression PR #135 fixed: typing "1500,50" must never report
  // 150050 (comma dropped, decimals read as extra integer digits).
  it('never reports the 100x amount a dropped comma used to produce', () => {
    expect(fractionalMoneyValue(analyzeMoneyInput('1500,50'))).not.toBe(150050);
    expect(fractionalMoneyValue(analyzeMoneyInput('1500,00'))).not.toBe(150000);
  });
});
