import { beforeEach, describe, expect, it, vi } from 'vitest';
import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { http, HttpResponse } from 'msw';
import { server } from '../../test/msw/server';
import { renderBooking } from '../../test/booking';
import { navigateTo } from '../../lib/navigation';
import { BOOKING_MESSAGES } from '../../messages';
import { PaymentFailedScreen } from './PaymentFailedScreen';

vi.mock('../../lib/navigation', () => ({ navigateTo: vi.fn() }));

const t = BOOKING_MESSAGES.publicBooking;

/** Records every body POSTed to a checkout route and answers with `answer`. */
function onCheckout(path: 'retry' | 'release', answer: () => Response) {
  const bodies: unknown[] = [];
  server.use(
    http.post(`*/book/checkout/${path}`, async ({ request }) => {
      bodies.push(await request.json());
      return answer();
    }),
  );
  return bodies;
}

beforeEach(() => {
  vi.mocked(navigateTo).mockClear();
});

describe('PaymentFailedScreen with the booking token MercadoPago sent back', () => {
  it('pays again on the same checkout', async () => {
    const bodies = onCheckout('retry', () =>
      HttpResponse.json({ checkout: { mp_init_point: 'https://mp.test/pay?pref_id=p1', expires_at: '2027-03-20T22:42:00Z' } }),
    );
    renderBooking(<PaymentFailedScreen slug="club-norte" token="link-token" />);

    await userEvent.click(screen.getByRole('button', { name: t.paymentFailedRetry }));

    await waitFor(() => {
      expect(navigateTo).toHaveBeenCalledWith('https://mp.test/pay?pref_id=p1');
    });
    expect(bodies).toEqual([{ token: 'link-token' }]);
  });

  it('says so when the booking can no longer be paid, and keeps the way to another time', async () => {
    onCheckout('retry', () =>
      HttpResponse.json({ type: 'https://vibe.com.ar/problems/conflict', title: 'Conflict', status: 409 }, { status: 409 }),
    );
    renderBooking(<PaymentFailedScreen slug="club-norte" token="link-token" />);

    await userEvent.click(screen.getByRole('button', { name: t.paymentFailedRetry }));

    expect(await screen.findByText(t.paymentFailedRetryGone)).toBeInTheDocument();
    expect(navigateTo).not.toHaveBeenCalled();
    expect(screen.queryByRole('button', { name: t.paymentFailedRetry })).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: t.paymentFailedChooseAnother })).toBeInTheDocument();
  });

});

describe('PaymentFailedScreen releasing the slot with the booking token', () => {
  it('gives the slot back before going to pick another time', async () => {
    const bodies = onCheckout('release', () => new HttpResponse(null, { status: 204 }));
    renderBooking(<PaymentFailedScreen slug="club-norte" token="link-token" />);

    await userEvent.click(screen.getByRole('button', { name: t.paymentFailedChooseAnother }));

    await waitFor(() => {
      expect(navigateTo).toHaveBeenCalledWith('/c/club-norte');
    });
    expect(bodies).toEqual([{ token: 'link-token' }]);
  });

  it('shows the booking when MercadoPago says it is paid', async () => {
    onCheckout('release', () =>
      HttpResponse.json(
        { type: 'https://vibe.com.ar/problems/booking-paid', title: 'Booking Paid', status: 409, detail: 'La reserva ya está pagada.' },
        { status: 409 },
      ),
    );
    renderBooking(<PaymentFailedScreen slug="club-norte" token="link-token" />);

    await userEvent.click(screen.getByRole('button', { name: t.paymentFailedChooseAnother }));

    await waitFor(() => {
      expect(navigateTo).toHaveBeenCalledWith('/c/club-norte/book/success?token=link-token');
    });
  });

  it('does not call a release that lost a write race paid, and keeps a way back to the complex', async () => {
    onCheckout('release', () =>
      HttpResponse.json({ type: 'https://vibe.com.ar/problems/conflict', title: 'Conflict', status: 409 }, { status: 409 }),
    );
    renderBooking(<PaymentFailedScreen slug="club-norte" token="link-token" />);

    await userEvent.click(screen.getByRole('button', { name: t.paymentFailedChooseAnother }));

    expect(await screen.findByText(t.paymentFailedActionError)).toBeInTheDocument();
    expect(navigateTo).not.toHaveBeenCalled();
    expect(screen.getByRole('link', { name: t.paymentFailedBackToComplex })).toHaveAttribute('href', '/c/club-norte');
  });

  it('keeps a way back to the complex when the release fails outright', async () => {
    onCheckout('release', () =>
      HttpResponse.json({ type: 'https://vibe.com.ar/problems/unavailable', title: 'Service Unavailable', status: 503 }, { status: 503 }),
    );
    renderBooking(<PaymentFailedScreen slug="club-norte" token="link-token" />);

    await userEvent.click(screen.getByRole('button', { name: t.paymentFailedChooseAnother }));

    expect(await screen.findByText(t.paymentFailedActionError)).toBeInTheDocument();
    expect(screen.getByRole('link', { name: t.paymentFailedBackToComplex })).toHaveAttribute('href', '/c/club-norte');
  });

  it('asks to wait while MercadoPago is still processing the payment, instead of saying it cannot be paid', async () => {
    const detail = 'Mercado Pago todavía está procesando tu pago. Esperá unos minutos.';
    onCheckout('retry', () =>
      HttpResponse.json(
        { type: 'https://vibe.com.ar/problems/payment-in-process', title: 'Payment In Process', status: 409, detail },
        { status: 409 },
      ),
    );
    renderBooking(<PaymentFailedScreen slug="club-norte" token="link-token" />);

    await userEvent.click(screen.getByRole('button', { name: t.paymentFailedRetry }));

    expect(await screen.findByText(detail)).toBeInTheDocument();
    expect(screen.queryByText(t.paymentFailedRetryGone)).not.toBeInTheDocument();
    expect(navigateTo).not.toHaveBeenCalled();
  });
});

describe('PaymentFailedScreen without a token', () => {
  it('only links back to the complex, as before', () => {
    renderBooking(<PaymentFailedScreen slug="club-norte" />);

    expect(screen.queryByRole('button', { name: t.paymentFailedRetry })).not.toBeInTheDocument();
    expect(screen.getByRole('link', { name: t.paymentFailedChooseAnother })).toHaveAttribute('href', '/c/club-norte');
  });
});
