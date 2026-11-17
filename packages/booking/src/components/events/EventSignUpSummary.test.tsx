import { describe, it, expect } from 'vitest';
import { screen, within } from '@testing-library/react';
import { renderBooking } from '../../test/booking';
import { EVENT, MEN_6_TO_8 } from '../../test/eventFixtures';
import { BOOKING_MESSAGES } from '../../messages';
import type { PublicOpenPlayEvent } from '../../api/types';
import { EventSignUpSummary } from './EventSignUpSummary';

describe('EventSignUpSummary title', () => {
  it('titles the event by its characteristics', () => {
    renderBooking(<EventSignUpSummary event={MEN_6_TO_8} />);

    expect(screen.getByRole('heading', { name: 'Cancha abierta de pádel · Caballeros · 6ta a 8va' })).toBeInTheDocument();
  });

  it('titles a fútbol event by its gender, in the words of the sport', () => {
    const soccer: PublicOpenPlayEvent = { ...EVENT, id: 'ev2', sport: 'soccer', modality: 'women', level_min: null, level_max: null };
    renderBooking(<EventSignUpSummary event={soccer} />);

    expect(screen.getByRole('heading', { name: 'Femenino' })).toBeInTheDocument();
  });
});

describe('EventSignUpSummary courts', () => {
  it('names one or two courts on a single line, then each fact once as a chip, like a booking ticket', () => {
    renderBooking(<EventSignUpSummary event={EVENT} />);

    const ticket = screen.getByRole('region', { name: BOOKING_MESSAGES.events.summaryLabel });
    expect(within(ticket).getByText(/Cancha.1, Cancha.2/)).toBeInTheDocument();
    for (const chip of ['Techada', 'Descubierta', 'Césped sintético', 'Con iluminación']) {
      expect(within(ticket).getAllByText(chip)).toHaveLength(1);
    }
  });

  it('counts the courts beyond two instead of naming each one, so the ticket stays short', () => {
    const [base] = EVENT.courts;
    if (!base) throw new Error('the fixture event needs a court');
    const courts = [0, 1, 2].map((i) => ({ ...base, id: `court-${String(i)}`, name: `Cancha ${String(i + 1)}` }));
    renderBooking(<EventSignUpSummary event={{ ...EVENT, courts }} />);

    const ticket = screen.getByRole('region', { name: BOOKING_MESSAGES.events.summaryLabel });
    expect(within(ticket).getByText(BOOKING_MESSAGES.events.courtsCount(3))).toBeInTheDocument();
    expect(within(ticket).queryByText(/Cancha.3/)).not.toBeInTheDocument();
  });

  it('shows no court list when the event has none', () => {
    renderBooking(<EventSignUpSummary event={{ ...EVENT, courts: [], court_names: [] }} />);
    expect(screen.queryByRole('listitem')).not.toBeInTheDocument();
  });
});

describe('EventSignUpSummary inside the checkout band', () => {
  it('draws no card of its own, so the band is the only surface', () => {
    renderBooking(<EventSignUpSummary event={EVENT} />);

    const ticket = screen.getByRole('region', { name: BOOKING_MESSAGES.events.summaryLabel });
    expect(ticket).not.toHaveClass('rounded-3xl');
    expect(ticket).not.toHaveClass('bg-primary-500');
  });
});
