import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import { PhoneInput } from './PhoneInput';

describe('PhoneInput — prefix box height matches the input', () => {
  // happy-dom (like jsdom) never runs layout: every element reports a zero
  // `getBoundingClientRect`, so a pixel-height assertion here would pass or
  // fail for reasons that have nothing to do with the actual bug (the "+54"
  // box rendering shorter than the input beside it on a phone). What we can
  // pin instead is the structure the height guarantee depends on: the row
  // stretches its children (`items-stretch`) and the prefix box has no
  // height of its own (`self-stretch`, no `h-*` class) — so it always
  // matches whatever height the input ends up with, at every breakpoint,
  // instead of two places having to repeat the same height class in sync.
  it('stretches the row and the prefix box instead of pinning a matching height class', () => {
    render(<PhoneInput value="" onChange={vi.fn()} />);

    const prefixBox = screen.getByText('+54');
    const row = prefixBox.parentElement;

    expect(row).toHaveClass('flex', 'items-stretch');
    expect(prefixBox).toHaveClass('self-stretch');
    // No fixed height of its own — that's what let it drift out of sync with
    // the input's height in the first place.
    expect(prefixBox.className).not.toMatch(/\bh-\d+\b/);
  });

  it('still lets a caller size the input without having to also size the prefix box', () => {
    render(<PhoneInput value="" onChange={vi.fn()} inputClassName="h-12 text-base" />);

    const input = screen.getByRole('textbox');
    expect(input).toHaveClass('h-12');
    // The prefix box picks up no height class of its own regardless.
    expect(screen.getByText('+54').className).not.toMatch(/\bh-\d+\b/);
  });
});
