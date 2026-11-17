import { describe, it, expect } from 'vitest';
import { screen } from '@testing-library/react';
import { renderBooking } from '../../test/booking';
import { EVENT } from '../../test/eventFixtures';
import { BOOKING_MESSAGES } from '../../messages';
import { eventRows } from './eventRows';
import { openPlayRows } from './openPlayAgendaModel';
import { EventsSection } from './EventsSection';

const t = BOOKING_MESSAGES.events;

describe('EventsSection', () => {
  it('titles the list "Eventos" and notes that it lists only the events with a place', () => {
    renderBooking(<EventsSection rows={eventRows(openPlayRows([EVENT]), [])} slug="test-club" />);

    expect(screen.getByRole('heading', { level: 3, name: t.pickerTitle })).toBeInTheDocument();
    expect(screen.getByText('Solo te mostramos eventos con lugar.')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: /Inscribirme/ })).toHaveAttribute('href', '/c/test-club/events/ev1');
  });

  it('has no picker of kinds and no heading of its own for open play or tournaments', () => {
    renderBooking(<EventsSection rows={eventRows(openPlayRows([EVENT]), [])} slug="test-club" />);

    expect(screen.queryByRole('button', { name: 'Canchas abiertas' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Torneos' })).not.toBeInTheDocument();
    expect(screen.queryByRole('heading', { name: 'Canchas abiertas' })).not.toBeInTheDocument();
    expect(screen.queryByRole('heading', { name: 'Torneos' })).not.toBeInTheDocument();
  });
});
