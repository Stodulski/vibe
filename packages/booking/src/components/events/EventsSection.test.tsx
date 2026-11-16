import { describe, it, expect } from 'vitest';
import { screen } from '@testing-library/react';
import { renderBooking } from '../../test/booking';
import { EVENT } from '../../test/eventFixtures';
import { BOOKING_MESSAGES } from '../../messages';
import { EventsSection } from './EventsSection';

const t = BOOKING_MESSAGES.events;

describe('EventsSection', () => {
  it('lists each event with its sign-up link, price per person and spots left', () => {
    renderBooking(<EventsSection events={[EVENT]} slug="test-club" />);

    expect(screen.getByRole('heading', { level: 3, name: t.sectionTitle })).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: EVENT.name })).toBeInTheDocument();
    expect(screen.getByText('19:00 a 22:00')).toBeInTheDocument();
    expect(screen.getByText('Cancha 1, Cancha 2')).toBeInTheDocument();
    expect(screen.getByText('Quedan 7 lugares')).toBeInTheDocument();
    expect(screen.getByText(/\$10\.000/)).toHaveTextContent('$10.000 por persona');
    expect(screen.queryByText(/cargo de servicio/i)).not.toBeInTheDocument();
    expect(screen.queryByText(/\$11\.000/)).not.toBeInTheDocument();
    expect(screen.getByRole('link', { name: t.signUp })).toHaveAttribute('href', '/c/test-club/events/ev1');
  });

  it('shows a full event as closed, without a sign-up link', () => {
    renderBooking(<EventsSection events={[{ ...EVENT, spots_left: 0 }]} slug="test-club" />);

    expect(screen.getByText(EVENT.name)).toBeInTheDocument();
    expect(screen.getAllByText(t.full)).toHaveLength(1);
    expect(screen.queryByRole('button', { name: t.full })).not.toBeInTheDocument();
    expect(screen.queryByRole('link', { name: t.signUp })).not.toBeInTheDocument();
  });
});

describe('EventsSection — events that offer no sign-up', () => {
  it('says Cancelado, not Completo, on a cancelled event even when it is also full', () => {
    renderBooking(<EventsSection events={[{ ...EVENT, status: 'cancelled', spots_left: 0 }]} slug="test-club" />);

    expect(screen.getByText(t.cancelledBadge)).toBeInTheDocument();
    expect(screen.queryByText(t.full)).not.toBeInTheDocument();
    expect(screen.queryByRole('link', { name: t.signUp })).not.toBeInTheDocument();
  });

  it('says the sign-up is closed, not Completo, on an event that already started', () => {
    renderBooking(<EventsSection events={[{ ...EVENT, date: '2000-01-01', spots_left: 0 }]} slug="test-club" />);

    expect(screen.getByText(t.closedBadge)).toBeInTheDocument();
    expect(screen.queryByText(t.full)).not.toBeInTheDocument();
    expect(screen.queryByRole('link', { name: t.signUp })).not.toBeInTheDocument();
  });
});
