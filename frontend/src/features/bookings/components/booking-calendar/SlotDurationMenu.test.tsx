import { describe, it, expect, vi } from 'vitest';
import { render } from '@testing-library/react';
import { SlotDurationMenu } from './SlotDurationMenu';
import { MIN_COLUMN_WIDTH_PX } from './gridLayout';
import type { OpenSlot } from './SlotDurationMenu';

/**
 * The menu's anchor is the one place left that multiplies a column index by
 * the column width. It is invisible and aria-hidden, so it is read from the
 * DOM rather than through a role: what matters is that it lands on the column
 * the click came from, at whatever width the grid settled on.
 */
function anchorOf(columnIndex: number, columnWidth: number): HTMLElement {
  const open: OpenSlot = {
    columnIndex,
    courtId: `ct${String(columnIndex + 1)}`,
    courtName: `Cancha ${String(columnIndex + 1)}`,
    slot: '10:00',
    durations: [60],
  };
  const { container } = render(
    <SlotDurationMenu open={open} columnWidth={columnWidth} onPreview={vi.fn()} onPick={vi.fn()} onClose={vi.fn()} />,
  );
  const anchor = container.querySelector<HTMLElement>('[aria-hidden="true"]');
  if (!anchor) throw new Error('the duration menu rendered no anchor');
  return anchor;
}

describe('SlotDurationMenu anchor', () => {
  it('follows a stretched column instead of the old fixed width', () => {
    // Two courts sharing 600px: the third column edge is at 600, not 400.
    expect(anchorOf(2, 300)).toHaveStyle({ left: '600px', width: '300px' });
  });

  it('still lands on the minimum-width column when the courts overflow', () => {
    expect(anchorOf(3, MIN_COLUMN_WIDTH_PX)).toHaveStyle({
      left: `${String(3 * MIN_COLUMN_WIDTH_PX)}px`,
      width: `${String(MIN_COLUMN_WIDTH_PX)}px`,
    });
  });

  it('puts the first column at the plot area origin', () => {
    expect(anchorOf(0, 600)).toHaveStyle({ left: '0px', width: '600px' });
  });
});
