import { describe, it, expect } from 'vitest';
// @vitest-environment node
import { makeUser, makeComplex } from '@/test/factories';
import {
  platformStatsResponseSchema,
  adminUsersResponseSchema,
  adminUserDetailResponseSchema,
  adminComplexesResponseSchema,
  adminComplexDetailResponseSchema,
} from './admin.schema';

describe('platformStatsResponseSchema', () => {
  it('validates adminApi.getStats response shape', () => {
    const result = platformStatsResponseSchema.safeParse({
      stats: {
        total_users: 10,
        active_users: 8,
        new_users_month: 2,
        total_complexes: 3,
        new_complexes_month: 1,
        total_courts: 6,
        total_bookings: 200,
        total_revenue: 1_000_000,
      },
    });
    expect(result.success).toBe(true);
  });
});

describe('adminUsersResponseSchema', () => {
  it('validates adminApi.getUsers response shape', () => {
    const result = adminUsersResponseSchema.safeParse({
      users: [
        {
          id: 'u1',
          email: 'user@test.com',
          first_name: 'Juan',
          last_name: 'Garcia',
          phone: '1155550000',
          role: 'owner',
          is_active: true,
          email_verified: true,
          created_at: '2026-01-01T00:00:00Z',
          complex_count: 1,
        },
      ],
      metadata: { has_more: false },
    });
    expect(result.success).toBe(true);
  });
});

describe('adminUserDetailResponseSchema', () => {
  it('validates adminApi.getUserDetail response shape', () => {
    const result = adminUserDetailResponseSchema.safeParse({ user: makeUser(), complexes: [makeComplex()] });
    expect(result.success).toBe(true);
  });
});

describe('adminComplexesResponseSchema', () => {
  it('validates adminApi.getComplexes response shape', () => {
    const result = adminComplexesResponseSchema.safeParse({
      complexes: [
        {
          id: 'c1',
          owner_id: 'u1',
          owner_name: 'Juan',
          owner_email: 'user@test.com',
          name: 'Club Padel',
          slug: 'club-padel',
          city: 'CABA',
          is_active: true,
          courts_count: 2,
          mp_connected: true,
          created_at: '2026-01-01T00:00:00Z',
        },
      ],
      metadata: { has_more: false },
    });
    expect(result.success).toBe(true);
  });
});

describe('adminComplexDetailResponseSchema', () => {
  it('validates adminApi.getComplexDetail response shape', () => {
    const result = adminComplexDetailResponseSchema.safeParse({
      complex: makeComplex(),
      owner_name: 'Juan',
      owner_email: 'user@test.com',
      courts_count: 2,
      clients_count: 20,
      bookings_count: 100,
      total_revenue: 500000,
    });
    expect(result.success).toBe(true);
  });
});
