import { describe, it, expect } from 'vitest';
// @vitest-environment node
import { makeCourt, makePrice } from '@/test/factories';
import {
  courtSchema,
  courtWithPricesSchema,
  courtsListEnvelopeSchema,
  courtEnvelopeSchema,
  pricesEnvelopeSchema,
  blockedSlotEnvelopeSchema,
  blockedSlotsEnvelopeSchema,
} from './court.schema';

const validBlockedSlot = {
  id: 'bs1',
  court_id: 'ct1',
  date: '2026-03-18',
  start_time: '10:00',
  end_time: '11:00',
  reason: 'Mantenimiento',
  created_at: '2026-01-01T00:00:00Z',
};

describe('courtSchema / courtWithPricesSchema', () => {
  it('validates a realistic Court fixture', () => {
    expect(courtSchema.safeParse(makeCourt()).success).toBe(true);
  });

  it('validates a Court with its prices attached', () => {
    const result = courtWithPricesSchema.safeParse({ ...makeCourt(), prices: [makePrice()] });
    expect(result.success).toBe(true);
  });
});

describe('courtsApi response envelopes', () => {
  it('validates courtsApi.list response shape', () => {
    const result = courtsListEnvelopeSchema.safeParse({
      courts: [{ ...makeCourt(), prices: [makePrice()] }],
    });
    expect(result.success).toBe(true);
  });

  it('validates courtsApi.create / update response shape', () => {
    expect(courtEnvelopeSchema.safeParse({ court: makeCourt() }).success).toBe(true);
  });

  it('validates courtsApi.updatePrices response shape', () => {
    expect(pricesEnvelopeSchema.safeParse({ prices: [makePrice()] }).success).toBe(true);
  });

  it('validates courtsApi.blockSlot response shape', () => {
    const result = blockedSlotEnvelopeSchema.safeParse({ blocked_slot: validBlockedSlot });
    expect(result.success).toBe(true);
  });

  it('validates courtsApi.listBlockedSlots response shape', () => {
    const result = blockedSlotsEnvelopeSchema.safeParse({ blocked_slots: [validBlockedSlot] });
    expect(result.success).toBe(true);
  });
});
