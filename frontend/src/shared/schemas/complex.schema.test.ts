import { describe, it, expect } from 'vitest';
// @vitest-environment node
import { makeComplex } from '@/test/factories';
import {
  complexSchema,
  scheduleSchema,
  blockedSlotSchema,
  complexesListEnvelopeSchema,
  complexEnvelopeSchema,
  deleteComplexResponseSchema,
  schedulesEnvelopeSchema,
  slugAvailableResponseSchema,
} from './complex.schema';

const validSchedule = {
  id: 'sch1',
  complex_id: 'c1',
  day: 'monday',
  open_time: '08:00',
  close_time: '23:00',
  is_closed: false,
};

describe('complexSchema', () => {
  it('validates a realistic Complex fixture', () => {
    expect(complexSchema.safeParse(makeComplex()).success).toBe(true);
  });

  it('validates a Complex with amenities and an optional court_count', () => {
    const result = complexSchema.safeParse(makeComplex({ amenities: ['wifi', 'parking'], court_count: 3 }));
    expect(result.success).toBe(true);
  });

  it('rejects an unknown amenity', () => {
    const result = complexSchema.safeParse(makeComplex({ amenities: ['pool' as never] }));
    expect(result.success).toBe(false);
  });
});

describe('scheduleSchema', () => {
  it('validates a realistic Schedule fixture', () => {
    expect(scheduleSchema.safeParse(validSchedule).success).toBe(true);
  });
});

describe('blockedSlotSchema', () => {
  it('validates a BlockedSlot with a null reason', () => {
    const result = blockedSlotSchema.safeParse({
      id: 'bs1',
      court_id: 'ct1',
      date: '2026-03-18',
      start_time: '10:00',
      end_time: '11:00',
      reason: null,
      created_at: '2026-01-01T00:00:00Z',
    });
    expect(result.success).toBe(true);
  });
});

describe('complexApi response schemas', () => {
  it('validates complexApi.list response shape', () => {
    expect(complexesListEnvelopeSchema.safeParse({ complexes: [makeComplex()] }).success).toBe(true);
  });

  it('validates complexApi.getById / create / update response shape', () => {
    expect(complexEnvelopeSchema.safeParse({ complex: makeComplex() }).success).toBe(true);
  });

  it('validates complexApi.delete response shape', () => {
    const result = deleteComplexResponseSchema.safeParse({ message: 'ok', courts_deactivated: 2 });
    expect(result.success).toBe(true);
  });

  it('validates complexApi.updateSchedules response shape', () => {
    expect(schedulesEnvelopeSchema.safeParse({ schedules: [validSchedule] }).success).toBe(true);
  });

  it('validates complexApi.slugAvailable response shape', () => {
    const result = slugAvailableResponseSchema.safeParse({
      slug: 'club-x',
      valid: true,
      available: false,
      suggestion: 'club-x-2',
    });
    expect(result.success).toBe(true);
  });
});
