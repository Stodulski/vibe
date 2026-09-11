// @vitest-environment node
import { describe, it, expect } from 'vitest';
import { queryKeys } from './queryKeys';

describe('queryKeys dashboard and misc', () => {
  describe('availability', () => {
    it('bySlugAndDate returns key with slug, date and duration', () => {
      expect(queryKeys.availability.bySlugAndDate('club', '2024-03-15', 90)).toEqual([
        'availability',
        'club',
        '2024-03-15',
        90,
      ]);
    });
  });

  describe('publicComplex', () => {
    it('bySlug returns key with slug', () => {
      expect(queryKeys.publicComplex.bySlug('club')).toEqual(['publicComplex', 'club']);
    });
  });

  describe('bookingStatus', () => {
    it('byId returns key with id', () => {
      expect(queryKeys.bookingStatus.byId('b-1')).toEqual(['bookingStatus', 'b-1']);
    });
  });

  describe('key uniqueness', () => {
    it('different entities produce different keys', () => {
      const courtsKey = queryKeys.courts.byComplex('c-1');
      const bookingsKey = queryKeys.bookings.byComplex('c-1');
      expect(courtsKey).not.toEqual(bookingsKey);
    });

    it('different IDs produce different keys', () => {
      const key1 = queryKeys.complexes.detail('c-1');
      const key2 = queryKeys.complexes.detail('c-2');
      expect(key1).not.toEqual(key2);
    });
  });
});
