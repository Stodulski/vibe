import { describe, it, expect } from 'vitest';
import { screen } from '@testing-library/react';
import { http, HttpResponse } from 'msw';
import { server } from '../../test/msw/server';
import { renderBooking } from '../../test/booking';
import { EVENT } from '../../test/eventFixtures';
import { BOOKING_MESSAGES } from '../../messages';
import { EventsSection } from './EventsSection';

const t = BOOKING_MESSAGES.events;

describe('EventsSection', () => {
  it('renders nothing when the complex has no upcoming events', async () => {
    const { container } = renderBooking(<EventsSection slug="test-club" />);
    await new Promise((resolve) => setTimeout(resolve, 50));
    expect(container).toBeEmptyDOMElement();
    expect(screen.queryByText(t.sectionTitle)).not.toBeInTheDocument();
  });

  it('renders nothing when the list fails to load', async () => {
    server.use(http.get('*/public/complexes/:slug/events', () => new HttpResponse(null, { status: 500 })));
    const { container } = renderBooking(<EventsSection slug="test-club" />);
    await new Promise((resolve) => setTimeout(resolve, 50));
    expect(container).toBeEmptyDOMElement();
  });

  it('lists each event with its sign-up link, price per person and spots left', async () => {
    server.use(http.get('*/public/complexes/:slug/events', () => HttpResponse.json({ events: [EVENT] })));
    renderBooking(<EventsSection slug="test-club" />);

    expect(await screen.findByRole('heading', { name: t.sectionTitle })).toBeInTheDocument();
    expect(screen.getByText(EVENT.name)).toBeInTheDocument();
    expect(screen.getByText('19:00 a 22:00 hs')).toBeInTheDocument();
    expect(screen.getByText('Cancha 1, Cancha 2')).toBeInTheDocument();
    expect(screen.getByText('Quedan 7 lugares')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: t.signUp })).toHaveAttribute('href', '/c/test-club/events/ev1');
  });

  it('shows a full event as closed, without a sign-up link', async () => {
    server.use(
      http.get('*/public/complexes/:slug/events', () =>
        HttpResponse.json({ events: [{ ...EVENT, spots_left: 0 }] }),
      ),
    );
    renderBooking(<EventsSection slug="test-club" />);

    expect(await screen.findByText(EVENT.name)).toBeInTheDocument();
    expect(screen.getAllByText(t.full).length).toBeGreaterThan(0);
    expect(screen.queryByRole('link', { name: t.signUp })).not.toBeInTheDocument();
  });
});
