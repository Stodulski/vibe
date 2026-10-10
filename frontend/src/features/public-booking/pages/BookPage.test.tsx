import { describe, it, expect, vi, afterEach } from 'vitest';
import { screen } from '@testing-library/react';
import { renderBooking, setBookingUrl } from '@/test/booking';
import { BOOKING_MESSAGES } from '../messages';
import BookPage from './BookPage';

afterEach(() => {
  vi.restoreAllMocks();
  setBookingUrl('/');
});

describe('BookPage', () => {
  it('redirects to the complex page when nothing failed', () => {
    const replace = vi.spyOn(window.location, 'replace').mockImplementation(() => undefined);
    setBookingUrl('/c/club-norte/book');

    renderBooking(<BookPage slug="club-norte" />);

    expect(replace).toHaveBeenCalledWith('/c/club-norte');
  });

  it('redirects on any other error, not only a failed payment', () => {
    const replace = vi.spyOn(window.location, 'replace').mockImplementation(() => undefined);
    setBookingUrl('/c/club-norte/book?error=slot_taken');

    renderBooking(<BookPage slug="club-norte" />);

    expect(replace).toHaveBeenCalledWith('/c/club-norte');
  });

  it('shows the payment failed screen with a way back to the complex page', () => {
    const replace = vi.spyOn(window.location, 'replace').mockImplementation(() => undefined);
    setBookingUrl('/c/club-norte/book?error=payment_failed');

    renderBooking(<BookPage slug="club-norte" />);

    expect(
      screen.getByRole('link', { name: BOOKING_MESSAGES.publicBooking.paymentFailedChooseAnother }),
    ).toHaveAttribute('href', '/c/club-norte');
    expect(replace).not.toHaveBeenCalled();
  });
});
