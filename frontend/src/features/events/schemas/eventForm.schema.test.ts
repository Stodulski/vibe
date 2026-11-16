import { describe, it, expect } from 'vitest';
import { ES_AR } from '@/shared/i18n/es_AR';
import { eventDurationMinutes, eventFormSchema } from './eventForm.schema';

const v = ES_AR.events.validation;

const valid = {
  sport: 'padel',
  modality: 'men' as const,
  allLevels: true,
  date: '2099-01-01',
  start_time: '19:00',
  end_time: '21:00',
  allCourts: true,
  courtIds: [] as string[],
  capacity: 20,
  price: 5000,
};

function messages(input: unknown): string[] {
  const result = eventFormSchema.safeParse(input);
  return result.success ? [] : result.error.issues.map((issue) => issue.message);
}

describe('eventDurationMinutes', () => {
  it('measures within a day', () => {
    expect(eventDurationMinutes('19:00', '21:30')).toBe(150);
  });

  it('reads an end at or before the start as the next day', () => {
    expect(eventDurationMinutes('22:00', '01:00')).toBe(180);
    expect(eventDurationMinutes('19:00', '19:00')).toBe(24 * 60);
  });
});

describe('eventFormSchema', () => {
  it('accepts a complete event for every level', () => {
    expect(eventFormSchema.safeParse(valid).success).toBe(true);
  });

  it('accepts exactly 30 minutes and exactly 12 hours, and refuses outside them', () => {
    expect(eventFormSchema.safeParse({ ...valid, end_time: '19:30' }).success).toBe(true);
    expect(eventFormSchema.safeParse({ ...valid, end_time: '07:00' }).success).toBe(true);
    expect(eventFormSchema.safeParse({ ...valid, end_time: '07:30' }).success).toBe(false);
    expect(eventFormSchema.safeParse({ ...valid, end_time: '19:00' }).success).toBe(false);
  });

  it.each([0, 501, 1.5, Number.NaN])('refuses a capacity of %s', (capacity) => {
    expect(eventFormSchema.safeParse({ ...valid, capacity }).success).toBe(false);
  });

  it.each([0, -1, Number.NaN])('refuses a price of %s: free events are not allowed', (price) => {
    expect(eventFormSchema.safeParse({ ...valid, price }).success).toBe(false);
  });

  it('needs a court once all courts is off', () => {
    expect(eventFormSchema.safeParse({ ...valid, allCourts: false }).success).toBe(false);
    expect(eventFormSchema.safeParse({ ...valid, allCourts: false, courtIds: ['ct1'] }).success).toBe(true);
  });

  it('needs a modality: the event is men’s, women’s or mixed', () => {
    expect(messages({ ...valid, modality: undefined })).toEqual([v.modalityRequired]);
  });

  // S15: the modality is every sport's, so a tennis or soccer event needs one too.
  it('needs a modality on a sport other than padel as well', () => {
    expect(messages({ ...valid, sport: 'tennis', modality: undefined })).toEqual([v.modalityRequired]);
    expect(messages({ ...valid, sport: 'soccer', modality: 'women' as const })).toEqual([]);
  });

  it('accepts one level as "solo 4ta" (both ends equal) and a range "6ta a 8va"', () => {
    expect(messages({ ...valid, allLevels: false, levelMin: 4, levelMax: 4 })).toEqual([]);
    expect(messages({ ...valid, allLevels: false, levelMin: 6, levelMax: 8 })).toEqual([]);
  });

  it('needs both ends of the range once all levels is off, and neither when it is on', () => {
    expect(messages({ ...valid, allLevels: false })).toEqual([v.levelRangeIncomplete]);
    expect(messages({ ...valid, allLevels: false, levelMin: 6 })).toEqual([v.levelRangeIncomplete]);
    expect(messages({ ...valid, allLevels: false, levelMax: 8 })).toEqual([v.levelRangeIncomplete]);
    expect(messages({ ...valid, allLevels: true, levelMin: 6, levelMax: 8 })).toEqual([]);
  });

  it('refuses a range whose "Desde" is weaker than its "Hasta"', () => {
    expect(messages({ ...valid, allLevels: false, levelMin: 8, levelMax: 6 })).toEqual([v.levelRangeOrder]);
  });

  it('keeps each level between 1ra and 9na', () => {
    expect(messages({ ...valid, allLevels: false, levelMin: 0, levelMax: 8 }).length).toBeGreaterThan(0);
    expect(messages({ ...valid, allLevels: false, levelMin: 6, levelMax: 10 }).length).toBeGreaterThan(0);
  });
});
