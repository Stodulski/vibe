import { describe, it, expect } from 'vitest';
// @vitest-environment node
import { blockSlotFormSchema } from './blockSlotForm.schema';

const validForm = {
  date: '2026-03-18',
  court_id: 'ct1',
  start_time: '10:00',
  end_time: '11:00',
  reason: 'Mantenimiento',
};

describe('blockSlotFormSchema', () => {
  it('accepts a fully valid form', () => {
    expect(blockSlotFormSchema.safeParse(validForm).success).toBe(true);
  });

  it('accepts a form with no reason, which is optional', () => {
    const { reason: _reason, ...withoutReason } = validForm;
    expect(blockSlotFormSchema.safeParse(withoutReason).success).toBe(true);
  });

  it('rejects an empty date', () => {
    expect(blockSlotFormSchema.safeParse({ ...validForm, date: '' }).success).toBe(false);
  });

  it('rejects no court selected', () => {
    expect(blockSlotFormSchema.safeParse({ ...validForm, court_id: '' }).success).toBe(false);
  });

  it('rejects an empty start_time', () => {
    expect(blockSlotFormSchema.safeParse({ ...validForm, start_time: '' }).success).toBe(false);
  });

  it('rejects an inverted time range, flagging end_time', () => {
    const result = blockSlotFormSchema.safeParse({ ...validForm, start_time: '11:00', end_time: '10:00' });
    expect(result.success).toBe(false);
    if (!result.success) {
      expect(result.error.issues[0]?.path).toEqual(['end_time']);
    }
  });

  it('rejects an equal start_time and end_time as an empty range', () => {
    const result = blockSlotFormSchema.safeParse({ ...validForm, start_time: '10:00', end_time: '10:00' });
    expect(result.success).toBe(false);
  });

  it('rejects a reason longer than 500 characters', () => {
    const result = blockSlotFormSchema.safeParse({ ...validForm, reason: 'a'.repeat(501) });
    expect(result.success).toBe(false);
  });
});
