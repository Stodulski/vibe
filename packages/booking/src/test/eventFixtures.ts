import type { PublicEventRegistration, PublicOpenPlayEvent } from '../api/types';

export const EVENT: PublicOpenPlayEvent = {
  id: 'ev1',
  name: 'Americano del sábado',
  sport: 'padel',
  date: '2099-12-05',
  start_time: '19:00',
  end_time: '22:00',
  duration_minutes: 180,
  capacity: 20,
  spots_left: 7,
  price: 1_000_000,
  service_fee: 100_000,
  total_price: 1_100_000,
  status: 'active',
  court_names: ['Cancha 1', 'Cancha 2'],
  complex_name: 'Club Norte',
  cancellation_hours: 24,
};

export function registration(overrides: Partial<PublicEventRegistration> = {}): PublicEventRegistration {
  return {
    status: 'confirmed',
    first_name: 'Ana',
    last_name: 'Pérez',
    price: 1_000_000,
    service_fee: 100_000,
    event: EVENT,
    complex_name: 'Club Norte',
    cancellation: { can_cancel: true, within_window: true, deadline: '2026-12-04T19:00:00-03:00' },
    ...overrides,
  };
}
