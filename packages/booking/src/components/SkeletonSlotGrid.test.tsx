import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { BOOKING_MESSAGES } from '../messages';
import { TIME_GROUPS } from './court-selector/constants';
import { SkeletonSlotGrid } from './SkeletonSlotGrid';

const t = BOOKING_MESSAGES;

describe('SkeletonSlotGrid', () => {
  it('announces the loading state through its status role', () => {
    render(<SkeletonSlotGrid />);
    expect(screen.getByRole('status')).toHaveAccessibleName(t.publicBooking.checkingAvailability);
  });

  it('depicts hour groups, not the old per-court cards', () => {
    // One grid of hour placeholders per part of the day, the shape
    // `CourtSelector`/`TimeGroupGrid` actually render, not a card per court.
    const { container } = render(<SkeletonSlotGrid />);
    expect(container.querySelectorAll('.grid')).toHaveLength(TIME_GROUPS.length);
  });
});
