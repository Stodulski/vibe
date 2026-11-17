import { describe, it, expect } from 'vitest';
import { screen, within } from '@testing-library/react';
import { http, HttpResponse } from 'msw';
import { server } from '../test/msw/server';
import { renderBooking, setBookingUrl } from '../test/booking';
import { EVENT } from '../test/eventFixtures';
import { BOOKING_MESSAGES } from '../messages';
import EventPage from './EventPage';

const t = BOOKING_MESSAGES;

function serveEvent(event = EVENT) {
  server.use(http.get('*/public/complexes/:slug/events/:id', () => HttpResponse.json({ event })));
}

function open() {
  setBookingUrl('/c/test-club/events/ev1');
  return renderBooking(<EventPage slug="test-club" eventId="ev1" />);
}

describe('EventPage checkout', () => {
  it('shows the open play checkout steps, with the court first and the data step on', async () => {
    serveEvent();
    open();

    const steps = await screen.findByRole('navigation', { name: t.publicBooking.stepProgress });
    expect(steps).toHaveTextContent(t.events.stepCourt);
    expect(steps).toHaveTextContent(t.publicBooking.stepPay);
    expect(steps.querySelector('[aria-current="step"]')).toHaveTextContent(t.publicBooking.stepData);
  });

  it('links back to the complex, and offers no "Ver el complejo"', async () => {
    serveEvent();
    open();

    expect(await screen.findByRole('link', { name: t.events.back })).toHaveAttribute('href', '/c/test-club');
    expect(screen.queryByRole('link', { name: t.events.backToComplex })).not.toBeInTheDocument();
  });

  it('shows the courts of the event on one line, like the court of a booking ticket', async () => {
    serveEvent();
    open();

    const ticket = await screen.findByRole('region', { name: t.events.summaryLabel });
    const line =
      EVENT.courts.length <= 2
        ? EVENT.courts.map((court) => court.name).join(', ')
        : t.events.courtsCount(EVENT.courts.length);
    expect(within(ticket).getByText((_, el) => (el?.textContent ?? '').replace(/\u00A0/g, ' ') === line)).toBeInTheDocument();
  });
});
