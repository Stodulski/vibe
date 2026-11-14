import { describe, it, expect } from 'vitest';
import { eventDurationMinutes, eventFormSchema } from './eventForm.schema';

const valid = {
  name: 'Americano',
  sport: 'padel',
  date: '2099-01-01',
  start_time: '19:00',
  end_time: '21:00',
  allCourts: true,
  courtIds: [] as string[],
  capacity: 20,
  price: 5000,
};

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
  it('accepts a complete event', () => {
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

  it('refuses a name of only spaces and one over 100 characters', () => {
    expect(eventFormSchema.safeParse({ ...valid, name: '   ' }).success).toBe(false);
    expect(eventFormSchema.safeParse({ ...valid, name: 'a'.repeat(101) }).success).toBe(false);
  });
});
