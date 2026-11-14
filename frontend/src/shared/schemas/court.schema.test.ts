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
    const result = blockedSlotsEnvelopeSchema.safeParse({
      blocked_slots: [validBlockedSlot],
      fixed_blocks: [],
      event_blocks: [],
    });
    expect(result.success).toBe(true);
  });

  it('reads the hours open-play events hold from event_blocks', () => {
    const eventBlock = {
      event_id: 'ev1',
      court_id: 'ct1',
      date: '2026-11-20',
      start_time: '19:00',
      end_time: '21:00',
      duration_minutes: 120,
      name: 'Americano de viernes',
    };
    const result = blockedSlotsEnvelopeSchema.safeParse({
      blocked_slots: [],
      fixed_blocks: [],
      event_blocks: [eventBlock],
    });
    expect(result.success && result.data.event_blocks).toEqual([eventBlock]);
  });

  it('refuses a response with no event_blocks list', () => {
    expect(blockedSlotsEnvelopeSchema.safeParse({ blocked_slots: [], fixed_blocks: [] }).success).toBe(false);
  });
});
