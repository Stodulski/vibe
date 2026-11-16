import { describe, it, expect } from 'vitest';
import { makeDetail } from '@/features/tournaments/testing/fixtures';
import type { OpenPlayEvent, TournamentOwnerSummary } from '@/shared/types/api.types';
import { buildAgenda } from './eventsHubAgenda';

function openPlay(id: string, date: string, startTime = '19:00'): OpenPlayEvent {
  return {
    id,
    complex_id: 'c1',
    name: id,
    sport: 'padel',
    date,
    start_time: startTime,
    end_time: '21:00',
    duration_minutes: 120,
    capacity: 8,
    price: 500000,
    status: 'active',
    courts: [{ id: 'ct1', name: 'Cancha 1' }],
    confirmed_count: 0,
    held_count: 0,
    spots_left: 8,
    created_at: '2026-11-14T12:00:00Z',
  };
}

function tournament(id: string, startDate: string): TournamentOwnerSummary {
  return { ...makeDetail().tournament, id, start_date: startDate, end_date: startDate, unscheduled_count: 0, categories: [] };
}

describe('buildAgenda', () => {
  it('lists upcoming items soonest first, across both kinds', () => {
    const items = buildAgenda([openPlay('ev-late', '2099-01-10')], [tournament('t-early', '2099-01-02')], 'upcoming');
    expect(items.map((item) => item.key)).toEqual(['tournament-t-early', 'openPlay-ev-late']);
  });

  it('lists past items most recent first, across both kinds', () => {
    const items = buildAgenda([openPlay('ev-old', '2020-01-01')], [tournament('t-new', '2020-06-01')], 'past');
    expect(items.map((item) => item.key)).toEqual(['tournament-t-new', 'openPlay-ev-old']);
  });

  it('breaks a same-day tie by start time', () => {
    const items = buildAgenda(
      [openPlay('late', '2099-01-02', '21:00'), openPlay('early', '2099-01-02', '19:00')],
      [],
      'upcoming',
    );
    expect(items.map((item) => item.key)).toEqual(['openPlay-early', 'openPlay-late']);
  });

  it('keeps each item tagged with its kind', () => {
    const [item] = buildAgenda([], [tournament('t1', '2099-01-02')], 'upcoming');
    expect(item?.kind).toBe('tournament');
  });
});
