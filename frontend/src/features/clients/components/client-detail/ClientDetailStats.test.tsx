import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { ClientDetailStats } from './ClientDetailStats';

// UI-03: the no-shows/attendance tones used to be a chained ternary built
// outside `cn()`. These pin the three bands on each side so the move to
// `noShowsTone`/`attendanceTone` didn't change which color renders for which
// value.
describe('ClientDetailStats', () => {
  it('reads a heavy no-show count as an error', () => {
    render(<ClientDetailStats totalBookings={10} noShows={3} attendance={100} />);
    expect(screen.getByText('3')).toHaveClass('text-error-text');
  });

  it('reads a light no-show count as a warning', () => {
    render(<ClientDetailStats totalBookings={10} noShows={1} attendance={100} />);
    expect(screen.getByText('1')).toHaveClass('text-warning-text');
  });

  it('reads zero no-shows as neutral', () => {
    render(<ClientDetailStats totalBookings={10} noShows={0} attendance={100} />);
    expect(screen.getByText('0')).toHaveClass('text-text-primary');
  });

  it('reads high attendance as a success', () => {
    render(<ClientDetailStats totalBookings={10} noShows={0} attendance={80} />);
    expect(screen.getByText('80%')).toHaveClass('text-success-text');
  });

  it('reads mid attendance as a warning', () => {
    render(<ClientDetailStats totalBookings={10} noShows={0} attendance={50} />);
    expect(screen.getByText('50%')).toHaveClass('text-warning-text');
  });

  it('reads low attendance as an error', () => {
    render(<ClientDetailStats totalBookings={10} noShows={0} attendance={49} />);
    expect(screen.getByText('49%')).toHaveClass('text-error-text');
  });
});
