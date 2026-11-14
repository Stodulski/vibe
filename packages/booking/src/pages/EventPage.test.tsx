import { describe, it, expect, vi, beforeEach } from 'vitest';
import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { http, HttpResponse } from 'msw';
import { server } from '../test/msw/server';
import { renderBooking, setBookingUrl } from '../test/booking';
import { EVENT, registration } from '../test/eventFixtures';
import { BOOKING_MESSAGES } from '../messages';
import { storeEventRegistrationToken } from '../lib/eventRegistrationStorage';
import { navigateTo } from '../lib/navigation';
import EventPage from './EventPage';

vi.mock('../lib/navigation', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../lib/navigation')>()),
  navigateTo: vi.fn(),
}));

const t = BOOKING_MESSAGES;

function serveEvent(event = EVENT) {
  server.use(http.get('*/public/complexes/:slug/events/:id', () => HttpResponse.json({ event })));
}

function serveRegistration(body: ReturnType<typeof registration>) {
  server.use(http.get('*/public/event-registrations/status', () => HttpResponse.json(body)));
}

function open(search = '') {
  setBookingUrl(`/c/test-club/events/ev1${search}`);
  return renderBooking(<EventPage slug="test-club" eventId="ev1" />);
}

beforeEach(() => {
  vi.mocked(navigateTo).mockClear();
});

describe('EventPage event', () => {
  it('shows the event with its price breakdown and the form', async () => {
    serveEvent();
    open();

    expect(await screen.findByRole('heading', { level: 1, name: EVENT.name })).toBeInTheDocument();
    expect(screen.getByLabelText(/^Nombre/)).toBeInTheDocument();
    expect(screen.getByLabelText(/^Apellido/)).toBeInTheDocument();
    expect(screen.getByLabelText(/^Teléfono/)).toBeInTheDocument();
    expect(screen.getByLabelText(/^Email/)).toBeInTheDocument();
    expect(screen.getByText(t.serviceFee.label)).toBeInTheDocument();
    expect(screen.getByRole('button', { name: t.publicBooking.paySecurely })).toBeInTheDocument();
  });

  it('shows a full event as closed with no form', async () => {
    serveEvent({ ...EVENT, spots_left: 0 });
    open();

    expect(await screen.findByText(t.events.fullDescription)).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: t.publicBooking.paySecurely })).not.toBeInTheDocument();
  });

  it('says a cancelled event was cancelled and offers no form', async () => {
    serveEvent({ ...EVENT, status: 'cancelled' });
    open();

    expect(await screen.findByRole('heading', { name: t.events.cancelledTitle })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: t.publicBooking.paySecurely })).not.toBeInTheDocument();
  });

  it('shows not found for an unknown event', async () => {
    server.use(http.get('*/public/complexes/:slug/events/:id', () => new HttpResponse(null, { status: 404 })));
    open();
    expect(await screen.findByText(t.events.eventNotFound)).toBeInTheDocument();
  });

});

describe('EventPage sign-up', () => {
  it('validates required fields before sending', async () => {
    serveEvent();
    const post = vi.fn();
    server.use(
      http.post('*/public/complexes/:slug/events/:id/registrations', () => {
        post();
        return HttpResponse.json({});
      }),
    );
    open();
    const user = userEvent.setup();
    await user.click(await screen.findByRole('button', { name: t.publicBooking.paySecurely }));

    expect(await screen.findByText(t.validation.nameRequired)).toBeInTheDocument();
    expect(post).not.toHaveBeenCalled();
  });

  it('sends the person, omits an empty email and goes to the Mercado Pago checkout', async () => {
    serveEvent();
    let body: unknown;
    server.use(
      http.post('*/public/complexes/:slug/events/:id/registrations', async ({ request }) => {
        body = await request.json();
        return HttpResponse.json(
          {
            registration: { status: 'held', held_until: '2026-12-05T19:15:00-03:00' },
            token: 'tok-1',
            event: EVENT,
            mp_init_point: 'https://mp.test/checkout',
            service_fee: 100_000,
            total_price: 1_100_000,
          },
          { status: 201 },
        );
      }),
    );
    open();
    const user = userEvent.setup();
    await user.type(await screen.findByLabelText(/^Nombre/), 'Ana');
    await user.type(screen.getByLabelText(/^Apellido/), 'Pérez');
    await user.type(screen.getByLabelText(/^Teléfono/), '1123456789');
    await user.click(screen.getByRole('button', { name: t.publicBooking.paySecurely }));

    await waitFor(() => {
      expect(navigateTo).toHaveBeenCalledWith('https://mp.test/checkout');
    });
    expect(body).toEqual({ first_name: 'Ana', last_name: 'Pérez', phone: '+541123456789' });
    expect(sessionStorage.getItem('vibe_event_registration:ev1')).toBe('"tok-1"');
  });

});

describe('EventPage refused sign-up', () => {
  it('explains a 409 without leaving the page', async () => {
    serveEvent();
    server.use(
      http.post('*/public/complexes/:slug/events/:id/registrations', () =>
        HttpResponse.json(
          { type: 'https://vibe.com.ar/problems/conflict', title: 'Conflict', status: 409, detail: 'the event is full' },
          { status: 409, headers: { 'Content-Type': 'application/problem+json' } },
        ),
      ),
    );
    open();
    const user = userEvent.setup();
    await user.type(await screen.findByLabelText(/^Nombre/), 'Ana');
    await user.type(screen.getByLabelText(/^Apellido/), 'Pérez');
    await user.type(screen.getByLabelText(/^Teléfono/), '1123456789');
    await user.click(screen.getByRole('button', { name: t.publicBooking.paySecurely }));

    expect(await screen.findByRole('alert')).toHaveTextContent(t.events.signUpConflict);
    expect(navigateTo).not.toHaveBeenCalled();
  });
});

describe('EventPage checkout return', () => {
  it('shows the processing state while the spot is still held', async () => {
    serveRegistration(registration({ status: 'held' }));
    open('?token=tok-1&registered=1');
    expect(await screen.findByText(t.events.processingTitle)).toBeInTheDocument();
  });

  it('shows the under review state for ?status=pending', async () => {
    serveRegistration(registration({ status: 'held' }));
    open('?token=tok-1&registered=1&status=pending');
    expect(await screen.findByText(t.events.underReviewTitle)).toBeInTheDocument();
  });

  it('confirms the spot and shows the refund window of the complex policy', async () => {
    serveRegistration(registration());
    open('?token=tok-1&registered=1');

    expect(await screen.findByText(t.events.confirmedTitle)).toBeInTheDocument();
    expect(screen.getByText(EVENT.name)).toBeInTheDocument();
    expect(screen.getByText(new RegExp(t.events.refundUntilPrefix))).toBeInTheDocument();
  });

  it('says the refund window passed when cancelling would keep the payment', async () => {
    serveRegistration(
      registration({ cancellation: { can_cancel: true, within_window: false, deadline: '2026-12-04T19:00:00-03:00' } }),
    );
    open('?token=tok-1&registered=1');
    expect(await screen.findByText(t.events.refundWindowPassed)).toBeInTheDocument();
  });

});

describe('EventPage checkout return, after the person acts', () => {
  it('cancels the spot and renders the server refund message as it comes', async () => {
    serveRegistration(registration());
    let cancelBody: unknown;
    server.use(
      http.post('*/public/event-registrations/cancel', async ({ request }) => {
        cancelBody = await request.json();
        return HttpResponse.json({
          registration: registration({ status: 'cancelled' }),
          refunded: true,
          refund: { status: 'issued', message: 'Te devolvimos lo que pagaste.', amount: 1_100_000 },
        });
      }),
    );
    open('?token=tok-1&registered=1');
    const user = userEvent.setup();
    await user.click(await screen.findByRole('button', { name: t.events.cancelSpot }));
    await user.click(await screen.findByRole('button', { name: t.events.cancelConfirmYes }));

    expect(await screen.findByText('Te devolvimos lo que pagaste.')).toBeInTheDocument();
    expect(cancelBody).toEqual({ token: 'tok-1' });
  });

  it('shows an expired spot with a way to sign up again', async () => {
    serveRegistration(registration({ status: 'expired', cancellation: { can_cancel: false, within_window: false, deadline: '2026-12-04T19:00:00-03:00' } }));
    open('?token=tok-1&registered=1');
    expect(await screen.findByText(t.events.expiredTitle)).toBeInTheDocument();
    expect(screen.getByRole('link', { name: t.events.expiredAction })).toHaveAttribute('href', '/c/test-club/events/ev1');
  });

  it('shows a link that does not resolve', async () => {
    server.use(http.get('*/public/event-registrations/status', () => new HttpResponse(null, { status: 404 })));
    open('?token=nope&registered=1');
    expect(await screen.findByText(t.events.registrationNotFoundDescription)).toBeInTheDocument();
  });

  it('on a failed payment releases the held spot with the stored token and brings the form back', async () => {
    serveEvent();
    storeEventRegistrationToken('ev1', 'tok-held');
    let cancelBody: unknown;
    server.use(
      http.post('*/public/event-registrations/cancel', async ({ request }) => {
        cancelBody = await request.json();
        return HttpResponse.json({
          registration: registration({ status: 'cancelled' }),
          refunded: false,
          refund: { status: 'none', message: 'Sin pagos para devolver.' },
        });
      }),
    );
    open('?error=payment_failed');
    const user = userEvent.setup();
    expect(await screen.findByText(t.publicBooking.paymentFailed)).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: t.events.paymentFailedAction }));

    expect(await screen.findByRole('button', { name: t.publicBooking.paySecurely })).toBeInTheDocument();
    expect(cancelBody).toEqual({ token: 'tok-held' });
    expect(window.location.search).toBe('');
  });
});
