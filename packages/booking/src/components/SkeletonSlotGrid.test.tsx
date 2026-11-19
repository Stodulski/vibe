import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { BOOKING_MESSAGES } from '../messages';
import { SkeletonSlotGrid } from './SkeletonSlotGrid';

const t = BOOKING_MESSAGES;

describe('SkeletonSlotGrid', () => {
  it('announces the loading state through its status role', () => {
    render(<SkeletonSlotGrid />);
    expect(screen.getByRole('status')).toHaveAccessibleName(t.publicBooking.checkingAvailability);
  });

  it('depicts the time ruler while loading', () => {
    render(<SkeletonSlotGrid />);
    expect(screen.getByTestId('ruler-skeleton')).toBeInTheDocument();
  });
});
