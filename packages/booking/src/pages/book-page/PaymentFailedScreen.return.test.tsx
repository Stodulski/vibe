import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { http, HttpResponse } from 'msw';
import { server } from '../../test/msw/server';
import { renderBooking } from '../../test/booking';
import { navigateTo } from '../../lib/navigation';
import { BOOKING_INFO_KEY } from '../../lib/storedBookingInfo';
import { BOOKING_MESSAGES } from '../../messages';
import { PaymentFailedScreen } from './PaymentFailedScreen';

vi.mock('../../lib/navigation', () => ({ navigateTo: vi.fn() }));

const t = BOOKING_MESSAGES.publicBooking;

// The slot whose payment was rejected: padel, 20 March, 10:00 for 90 minutes.
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
  sport: 'padel',
};
const HOURS_OF_THAT_DAY = '/c/club-norte?date=2026-03-20&duration=90&sport=padel#horarios';

beforeEach(() => {
  vi.mocked(navigateTo).mockClear();
  window.sessionStorage.setItem(BOOKING_INFO_KEY, JSON.stringify(storedBooking));
});

afterEach(() => {
  window.sessionStorage.clear();
});

describe('PaymentFailedScreen back to the hours of the rejected slot', () => {
  it('lands on the hours of that day, sport and duration after giving the slot back', async () => {
    server.use(http.post('*/book/checkout/release', () => new HttpResponse(null, { status: 204 })));
    renderBooking(<PaymentFailedScreen slug="club-norte" token="link-token" />);

    await userEvent.click(screen.getByRole('button', { name: t.paymentFailedChooseAnother }));

    await waitFor(() => {
      expect(navigateTo).toHaveBeenCalledWith(HOURS_OF_THAT_DAY);
    });
  });

  it('links to the hours of that day when the redirect carried no token', () => {
    renderBooking(<PaymentFailedScreen slug="club-norte" />);

    expect(screen.getByRole('link', { name: t.paymentFailedChooseAnother })).toHaveAttribute('href', HOURS_OF_THAT_DAY);
  });

  it('falls back to the complex page when the slot is not stored', async () => {
    window.sessionStorage.clear();
    server.use(http.post('*/book/checkout/release', () => new HttpResponse(null, { status: 204 })));
    renderBooking(<PaymentFailedScreen slug="club-norte" token="link-token" />);

    await userEvent.click(screen.getByRole('button', { name: t.paymentFailedChooseAnother }));

    await waitFor(() => {
      expect(navigateTo).toHaveBeenCalledWith('/c/club-norte');
    });
  });
});

describe('PaymentFailedScreen way back to the complex', () => {
  it('offers it from the start, before any action fails', () => {
    renderBooking(<PaymentFailedScreen slug="club-norte" token="link-token" />);

    expect(screen.getByRole('link', { name: t.paymentFailedBackToComplex })).toHaveAttribute('href', '/c/club-norte');
  });
});
