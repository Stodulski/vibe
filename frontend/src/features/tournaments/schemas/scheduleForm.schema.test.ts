import { describe, it, expect } from 'vitest';
import { ES_AR } from '@/shared/i18n/es_AR';
import { makeScheduleFormSchema } from './scheduleForm.schema';

const t = ES_AR.tournaments.schedule;
const schema = makeScheduleFormSchema('2026-12-05', '2026-12-07');
const valid = { court_id: 'ct1', date: '2026-12-06', start_time: '18:00', duration_minutes: 90 };

function messages(values: object): string[] {
  const result = schema.safeParse(values);
  return result.success ? [] : result.error.issues.map((issue) => issue.message);
}

describe('makeScheduleFormSchema', () => {
  it('accepts a day inside the tournament, on its first and last day too', () => {
    expect(messages(valid)).toEqual([]);
    expect(messages({ ...valid, date: '2026-12-05' })).toEqual([]);
    expect(messages({ ...valid, date: '2026-12-07' })).toEqual([]);
  });

  it('refuses a day outside the tournament days', () => {
    expect(messages({ ...valid, date: '2026-12-04' })).toEqual([t.outsideDays]);
    expect(messages({ ...valid, date: '2026-12-08' })).toEqual([t.outsideDays]);
  });

  it('asks for the court, day and time', () => {
    expect(messages({ ...valid, court_id: '', date: '', start_time: '' })).toEqual([
      t.courtRequired,
      ES_AR.validation.dateRequired,
      ES_AR.validation.timeRequired,
    ]);
  });

  it('keeps the duration between 30 and 480 minutes', () => {
    expect(messages({ ...valid, duration_minutes: 20 })).toEqual([t.durationRange]);
    expect(messages({ ...valid, duration_minutes: 500 })).toEqual([t.durationRange]);
    expect(messages({ ...valid, duration_minutes: Number.NaN })).toEqual([t.durationRange]);
    expect(messages({ ...valid, duration_minutes: 480 })).toEqual([]);
  });
});
