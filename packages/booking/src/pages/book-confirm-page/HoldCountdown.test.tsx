import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { act, render, screen } from '@testing-library/react';
import { HoldCountdown } from './HoldCountdown';
import { formatRemaining } from './holdTime';
import { BOOKING_MESSAGES } from '../../messages';

const t = BOOKING_MESSAGES;

const NOW = new Date('2030-01-01T12:00:00.000Z');

function at(seconds: number): string {
  return new Date(NOW.getTime() + seconds * 1000).toISOString();
}

beforeEach(() => {
  vi.useFakeTimers();
  vi.setSystemTime(NOW);
});

afterEach(() => {
  vi.useRealTimers();
});

describe('formatRemaining', () => {
  it.each([
    [900, '15:00'],
    [605, '10:05'],
    [59, '00:59'],
    [0, '00:00'],
  ])('formats %i seconds as %s', (seconds, text) => {
    expect(formatRemaining(seconds)).toBe(text);
  });
});

describe('HoldCountdown', () => {
  it('counts down every second from the server expiry', () => {
    render(<HoldCountdown expiresAt={at(90)} onExpire={vi.fn()} />);
    expect(screen.getByRole('timer')).toHaveTextContent('01:30');

    act(() => {
      vi.advanceTimersByTime(1000);
    });
    expect(screen.getByRole('timer')).toHaveTextContent('01:29');
  });

  it('calls onExpire once when it reaches zero', () => {
    const onExpire = vi.fn();
    render(<HoldCountdown expiresAt={at(2)} onExpire={onExpire} />);
    expect(onExpire).not.toHaveBeenCalled();

    act(() => {
      vi.advanceTimersByTime(2000);
    });
    expect(screen.getByRole('timer')).toHaveTextContent('00:00');
    act(() => {
      vi.advanceTimersByTime(3000);
    });
    expect(onExpire).toHaveBeenCalledTimes(1);
  });

  it('expires at once when the hold is already over', () => {
    const onExpire = vi.fn();
    render(<HoldCountdown expiresAt={at(-30)} onExpire={onExpire} />);
    expect(onExpire).toHaveBeenCalledTimes(1);
  });

  it('announces only at the thresholds, not every second', () => {
    render(<HoldCountdown expiresAt={at(5 * 60 + 2)} onExpire={vi.fn()} />);
    const live = screen.getByTestId('hold-countdown-announcement');
    expect(live).toHaveAttribute('aria-live', 'polite');
    expect(live).toBeEmptyDOMElement();
    // The ticking text itself is never a live region.
    expect(screen.getByRole('timer')).not.toHaveAttribute('aria-live', 'polite');

    act(() => {
      vi.advanceTimersByTime(1000);
    });
    expect(live).toBeEmptyDOMElement();

    act(() => {
      vi.advanceTimersByTime(2000);
    });
    expect(live).toHaveTextContent(t.publicBooking.holdFiveMinutesLeft);

    act(() => {
      vi.advanceTimersByTime(4 * 60 * 1000);
    });
    expect(live).toHaveTextContent(t.publicBooking.holdOneMinuteLeft);
  });
});
