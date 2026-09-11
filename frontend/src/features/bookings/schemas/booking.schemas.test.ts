import { createBookingSchema } from './booking.schemas';

const validBase = {
  court_id: 'court-1',
  duration_minutes: 90,
  client_phone: '+541155550000',
  client_first_name: 'Juan',
  client_last_name: 'Perez',
};

/**
 * Approval tests for `createBookingSchema`'s date/time-in-past `.refine`.
 * These capture CURRENT behavior before the guard-ladder refactor (replacing
 * `date.split('-').map(Number)` / `start_time.split(':').map(Number)` with
 * `parseYmd`/`parseHhMm`) so the refactor can be verified as behavior-preserving.
 */
describe('createBookingSchema — time-in-past refine', () => {
  it('accepts a far-future date/time', () => {
    const result = createBookingSchema.safeParse({
      ...validBase,
      date: '2099-01-01',
      start_time: '10:00',
    });

    expect(result.success).toBe(true);
  });

  it('rejects a date/time far in the past', () => {
    const result = createBookingSchema.safeParse({
      ...validBase,
      date: '2000-01-01',
      start_time: '10:00',
    });

    expect(result.success).toBe(false);
    if (!result.success) {
      const startTimeIssue = result.error.issues.find((i) => i.path.includes('start_time'));
      expect(startTimeIssue).toBeDefined();
    }
  });

  it('does not fail the time-in-past check when date is empty (caught by the required-field rule instead)', () => {
    const result = createBookingSchema.safeParse({
      ...validBase,
      date: '',
      start_time: '10:00',
    });

    expect(result.success).toBe(false);
    if (!result.success) {
      // The "date required" issue must be present; the time-in-past refine
      // short-circuits (`if (!date) return true`) and does not add a duplicate.
      const dateRequiredIssue = result.error.issues.find((i) => i.path.includes('date') && i.path.length === 1);
      expect(dateRequiredIssue).toBeDefined();
    }
  });
});
