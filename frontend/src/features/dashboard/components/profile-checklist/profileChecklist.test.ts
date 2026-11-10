import { describe, it, expect } from 'vitest';
import { buildProfileChecklist, MIN_DESCRIPTION_LENGTH, type ProfileItemId } from './profileChecklist';
import { makeActivePricedCourt, makeCompleteComplex, makeCourt, makePrice } from '@/test/factories';
import type { Complex, CourtWithPrices } from '@/shared/types/api.types';

interface Input {
  complex: Complex;
  courts: CourtWithPrices[];
}

function doneById(input: Input): Record<ProfileItemId, boolean> {
  return Object.fromEntries(buildProfileChecklist(input).map((item) => [item.id, item.done])) as Record<
    ProfileItemId,
    boolean
  >;
}

/** Everything done: a complete complex and a priced active court. */
function completeInput(): Input {
  return {
    complex: makeCompleteComplex(),
    courts: [makeActivePricedCourt()],
  };
}

describe('buildProfileChecklist', () => {
  it('lists the eight profile items in the order the owner fills them, MercadoPago last', () => {
    const ids = buildProfileChecklist(completeInput()).map((item) => item.id);

    expect(ids).toEqual([
      'cover',
      'logo',
      'description',
      'amenities',
      'location',
      'phone',
      'pricedCourt',
      'mercadoPago',
    ]);
  });

  it('marks every item done for a complete profile with a priced court', () => {
    const done = doneById(completeInput());

    expect(Object.values(done).every(Boolean)).toBe(true);
  });

  it.each<[ProfileItemId, Partial<Complex>]>([
    ['cover', { cover_url: null }],
    ['cover', { cover_url: '   ' }],
    ['logo', { logo_url: null }],
    ['description', { description: null }],
    ['description', { description: '   ' }],
    ['description', { description: 'x'.repeat(MIN_DESCRIPTION_LENGTH - 1) }],
    ['amenities', { amenities: [] }],
    ['location', { latitude: null }],
    ['location', { longitude: null }],
    ['phone', { phone: '  ' }],
    ['mercadoPago', { payments_enabled: false }],
  ])('leaves %s open when the complex has %o', (id, override) => {
    const done = doneById({ ...completeInput(), complex: makeCompleteComplex(override) });

    expect(done[id]).toBe(false);
  });

  it('counts a description of exactly the minimum length, ignoring surrounding spaces', () => {
    const padded = `   ${'x'.repeat(MIN_DESCRIPTION_LENGTH)}   `;

    const done = doneById({ ...completeInput(), complex: makeCompleteComplex({ description: padded }) });

    expect(done.description).toBe(true);
  });

  it('counts a location at zero latitude as set', () => {
    const done = doneById({ ...completeInput(), complex: makeCompleteComplex({ latitude: 0, longitude: 0 }) });

    expect(done.location).toBe(true);
  });
});

describe('buildProfileChecklist: courts', () => {
  it('leaves the court item open when there are no courts', () => {
    expect(doneById({ ...completeInput(), courts: [] }).pricedCourt).toBe(false);
  });

  it('leaves the court item open when the only court is inactive', () => {
    const inactive = { ...makeActivePricedCourt(), is_active: false };

    expect(doneById({ ...completeInput(), courts: [inactive] }).pricedCourt).toBe(false);
  });

  it('leaves the court item open when an active court has no price', () => {
    const unpriced = { ...makeCourt({ is_active: true }), prices: [] };

    expect(doneById({ ...completeInput(), courts: [unpriced] }).pricedCourt).toBe(false);
  });

  it('leaves the court item open when the only price is zero', () => {
    const free = { ...makeCourt({ is_active: true }), prices: [makePrice({ price: 0 })] };

    expect(doneById({ ...completeInput(), courts: [free] }).pricedCourt).toBe(false);
  });
});
