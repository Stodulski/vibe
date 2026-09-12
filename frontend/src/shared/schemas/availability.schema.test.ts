// @vitest-environment node
import { availabilityDataSchema, availabilityEnvelopeSchema } from './availability.schema';

const validAvailability = {
  date: '2026-03-18',
  day: 'wednesday',
  is_open: true,
  courts: [
    {
      court_id: 'ct1',
      court_name: 'Cancha 1',
      sport: 'padel',
      court_type: 'outdoor',
      duration_minutes: 60,
      slots: [
        { start_time: '10:00', end_time: '11:00', start_min: 600, duration_minutes: 60, price: 5000, available: true },
      ],
    },
  ],
};

describe('availabilityDataSchema', () => {
  it('validates a realistic AvailabilityData fixture', () => {
    expect(availabilityDataSchema.safeParse(validAvailability).success).toBe(true);
  });

  it('accepts a court with no description', () => {
    const result = availabilityDataSchema.safeParse(validAvailability);
    expect(result.success).toBe(true);
  });
});

describe('availabilityDataSchema — unknown vocabulary', () => {
  it('keeps the whole grid when one court is in a sport this build predates', () => {
    // The public slot grid is the revenue path: an unknown sport must cost
    // that court its label, not the day's availability.
    const withNewSport = {
      ...validAvailability,
      day: 'quintaday',
      courts: [
        { ...validAvailability.courts[0], sport: 'pickleball', court_type: 'covered_roof' },
        ...validAvailability.courts.slice(1),
      ],
    };
    const result = availabilityDataSchema.safeParse(withNewSport);
    expect(result.success).toBe(true);
    expect(result.success && result.data.courts[0]?.sport).toBe('pickleball');
  });
});

describe('availabilityEnvelopeSchema', () => {
  it('validates publicBookingApi.getAvailability response shape', () => {
    const result = availabilityEnvelopeSchema.safeParse({ availability: validAvailability });
    expect(result.success).toBe(true);
  });
});
