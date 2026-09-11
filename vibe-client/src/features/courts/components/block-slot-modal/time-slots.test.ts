import { describe, it, expect } from 'vitest';
import { timeToMinutes, generateTimeSlots } from './time-slots';

describe('timeToMinutes', () => {
  it('converts a HH:MM string to minutes since midnight', () => {
    expect(timeToMinutes('09:30')).toBe(570);
  });

  it('converts midnight to 0', () => {
    expect(timeToMinutes('00:00')).toBe(0);
  });
});

describe('generateTimeSlots', () => {
  it('generates 30-minute slots for the default full-day range', () => {
    const slots = generateTimeSlots();
    expect(slots[0]).toBe('00:00');
    expect(slots[slots.length - 1]).toBe('23:30');
    expect(slots).toHaveLength(48);
  });

  it('generates slots for a bounded range', () => {
    expect(generateTimeSlots('08:00', '10:00')).toEqual(['08:00', '08:30', '09:00', '09:30']);
  });
});
