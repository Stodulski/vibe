// @vitest-environment node
import { describe, it, expect } from 'vitest';
import { queryKeys } from './queryKeys';

describe('queryKeys entities', () => {
  describe('courts', () => {
    it('byComplex returns key with complexId', () => {
      expect(queryKeys.courts.byComplex('c-1')).toEqual(['courts', 'c-1']);
    });

    it('detail returns key with court id', () => {
      expect(queryKeys.courts.detail('ct-1')).toEqual(['courts', 'ct-1']);
    });
  });

  describe('bookings', () => {
    it('byComplex returns key with complexId', () => {
      expect(queryKeys.bookings.byComplex('c-1')).toEqual(['bookings', 'c-1']);
    });

    it('byDate returns key with complexId and date', () => {
      expect(queryKeys.bookings.byDate('c-1', '2024-03-15')).toEqual(['bookings', 'c-1', '2024-03-15']);
    });

    it('detail returns key with booking id', () => {
      expect(queryKeys.bookings.detail('b-1')).toEqual(['bookings', 'detail', 'b-1']);
    });
  });

  describe('clients', () => {
    it('byComplex returns key with complexId', () => {
      expect(queryKeys.clients.byComplex('c-1')).toEqual(['clients', 'c-1']);
    });

    it('detail returns key with complexId and clientId', () => {
      expect(queryKeys.clients.detail('c-1', 'cl-1')).toEqual(['clients', 'c-1', 'cl-1']);
    });
  });
});
