import { afterEach, describe, expect, it } from 'vitest';
import { screen } from '@testing-library/react';
import { renderBooking } from '../../test/booking';
import { BOOKING_MESSAGES } from '../../messages';
import { BOOKING_INFO_KEY } from '../../lib/storedBookingInfo';
import { PaymentFailedScreen } from './PaymentFailedScreen';

const storedBooking = {
  courtName: 'Cancha 1',
  date: '2026-03-20',
  startTime: '10:00',
  startsAt: '2026-03-20T10:00:00-03:00',
  endsAt: '2026-03-20T11:30:00-03:00',
  price: 1_000_000,
  depositAmount: 300_000,
  complexName: 'Club Norte',
  complexPhone: '+541155550000',
  cancellationHours: 24,
  clientPhone: '1123456789',
};

afterEach(() => {
  window.sessionStorage.clear();
});

describe('PaymentFailedScreen contact actions', () => {
  it('offers WhatsApp to the club before the phone call when the failed booking is stored', () => {
    window.sessionStorage.setItem(BOOKING_INFO_KEY, JSON.stringify(storedBooking));

    renderBooking(<PaymentFailedScreen slug="club-norte" />);

    const whatsApp = screen.getByRole('link', { name: 'Escribir por WhatsApp a Club Norte' });
    expect(whatsApp).toHaveAttribute('href', 'https://wa.me/541155550000');
    expect(whatsApp).toHaveAttribute('target', '_blank');
    expect(whatsApp).toHaveTextContent(BOOKING_MESSAGES.publicBooking.paymentFailedWhatsApp);
    expect(screen.getByRole('link', { name: BOOKING_MESSAGES.publicBooking.paymentFailedCallComplex })).toHaveAttribute(
      'href',
      'tel:+541155550000',
    );
  });

  it('offers no WhatsApp link when there is no stored booking to point it at', () => {
    renderBooking(<PaymentFailedScreen slug="club-norte" />);

    expect(screen.queryByRole('link', { name: /WhatsApp/ })).not.toBeInTheDocument();
    expect(screen.queryByRole('link', { name: BOOKING_MESSAGES.publicBooking.paymentFailedCallComplex })).not.toBeInTheDocument();
  });
});
