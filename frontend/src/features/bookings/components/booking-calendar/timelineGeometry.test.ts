import { describe, it, expect } from 'vitest';
import { barPosition, visibleRange, snapDownToSlot } from './timelineGeometry';

describe('barPosition', () => {
  const rangeStartMin = 8 * 60; // 08:00
  const rangeEndMin = 14 * 60; // 14:00 -> 360min span

  it('positions a booking at the very start of the range', () => {
    const { leftPct, widthPct } = barPosition(rangeStartMin, rangeStartMin + 60, rangeStartMin, rangeEndMin);
    expect(leftPct).toBe(0);
    expect(widthPct).toBeCloseTo((60 / 360) * 100);
  });

  it('positions a booking at the very end of the range', () => {
    const { leftPct, widthPct } = barPosition(rangeEndMin - 60, rangeEndMin, rangeStartMin, rangeEndMin);
    expect(leftPct).toBeCloseTo((300 / 360) * 100);
    expect(widthPct).toBeCloseTo((60 / 360) * 100);
  });

  it('positions a booking spanning the whole range', () => {
    const { leftPct, widthPct } = barPosition(rangeStartMin, rangeEndMin, rangeStartMin, rangeEndMin);
    expect(leftPct).toBe(0);
    expect(widthPct).toBe(100);
  });

  it('guards endMin <= startMin by treating it as a 30-minute bar', () => {
    const start = rangeStartMin + 60;
    const { widthPct } = barPosition(start, start, rangeStartMin, rangeEndMin);
    expect(widthPct).toBeCloseTo((30 / 360) * 100);

    const { widthPct: widthPctBefore } = barPosition(start, start - 10, rangeStartMin, rangeEndMin);
    expect(widthPctBefore).toBeCloseTo((30 / 360) * 100);
  });

  it('clamps a booking starting before rangeStartMin instead of a negative left', () => {
    const { leftPct, widthPct } = barPosition(rangeStartMin - 90, rangeStartMin + 30, rangeStartMin, rangeEndMin);
    expect(leftPct).toBe(0);
    expect(widthPct).toBeCloseTo((30 / 360) * 100);
  });
});

describe('visibleRange', () => {
  it('rounds open/close minutes out to the nearest full hour', () => {
    expect(visibleRange(8 * 60, 22 * 60 + 30)).toEqual({
      rangeStartMin: 8 * 60,
      rangeEndMin: 23 * 60,
    });
    expect(visibleRange(8 * 60 + 15, 22 * 60)).toEqual({
      rangeStartMin: 8 * 60,
      rangeEndMin: 22 * 60,
    });
  });
});

describe('snapDownToSlot', () => {
  const rangeStartMin = 8 * 60;
  const rangeEndMin = 23 * 60;

  it('snaps down to the nearest 30-minute slot', () => {
    expect(snapDownToSlot(9 * 60 + 47, rangeStartMin, rangeEndMin)).toBe(9 * 60 + 30);
  });

  it('clamps below the range start', () => {
    expect(snapDownToSlot(7 * 60, rangeStartMin, rangeEndMin)).toBe(rangeStartMin);
  });

  it('clamps above the range end', () => {
    expect(snapDownToSlot(23 * 60 + 45, rangeStartMin, rangeEndMin)).toBe(rangeEndMin);
  });
});
