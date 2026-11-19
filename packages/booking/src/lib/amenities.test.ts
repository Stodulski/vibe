import { describe, expect, it } from 'vitest';
import { orderedAmenities } from './amenities';

describe('amenities', () => {
  it('lists a selected new amenity in the canonical order', () => {
    expect(orderedAmenities(['kids_area', 'parking']).map((a) => a.value)).toEqual(['parking', 'kids_area']);
  });
});
