import { z } from 'zod';
import type {
  PlatformStats,
  PlatformStatsResponse,
  AdminUserRow,
  AdminUsersResponse,
  AdminUserDetailResponse,
  AdminComplexRow,
  AdminComplexesResponse,
  AdminComplexDetailResponse,
} from '@/shared/types/api.types';
import { paginationMetadataSchema } from './envelope.schema';
import { userRoleSchema, userSchema } from './auth.schema';
import { complexSchema } from './complex.schema';

// ─── Admin ───

const platformStatsSchema = z
  .object({
    total_users: z.number(),
    active_users: z.number(),
    new_users_month: z.number(),
    total_complexes: z.number(),
    new_complexes_month: z.number(),
    total_courts: z.number(),
    total_bookings: z.number(),
    total_revenue: z.number(),
  })
  .loose() satisfies z.ZodType<PlatformStats>;

export const platformStatsResponseSchema = z
  .object({
    stats: platformStatsSchema,
  })
  .loose() satisfies z.ZodType<PlatformStatsResponse>;

const adminUserRowSchema = z
  .object({
    id: z.string(),
    email: z.string(),
    first_name: z.string(),
    last_name: z.string(),
    phone: z.string(),
    role: userRoleSchema,
    is_active: z.boolean(),
    email_verified: z.boolean(),
    created_at: z.string(),
    complex_count: z.number(),
  })
  .loose() satisfies z.ZodType<AdminUserRow>;

export const adminUsersResponseSchema = z
  .object({
    users: z.array(adminUserRowSchema),
    metadata: paginationMetadataSchema,
  })
  .loose() satisfies z.ZodType<AdminUsersResponse>;

export const adminUserDetailResponseSchema = z
  .object({
    user: userSchema,
    complexes: z.array(complexSchema),
  })
  .loose() satisfies z.ZodType<AdminUserDetailResponse>;

const adminComplexRowSchema = z
  .object({
    id: z.string(),
    owner_id: z.string(),
    owner_name: z.string(),
    owner_email: z.string(),
    name: z.string(),
    slug: z.string(),
    city: z.string(),
    is_active: z.boolean(),
    courts_count: z.number(),
    mp_connected: z.boolean(),
    created_at: z.string(),
  })
  .loose() satisfies z.ZodType<AdminComplexRow>;

export const adminComplexesResponseSchema = z
  .object({
    complexes: z.array(adminComplexRowSchema),
    metadata: paginationMetadataSchema,
  })
  .loose() satisfies z.ZodType<AdminComplexesResponse>;

export const adminComplexDetailResponseSchema = z
  .object({
    complex: complexSchema,
    owner_name: z.string(),
    owner_email: z.string(),
    courts_count: z.number(),
    clients_count: z.number(),
    bookings_count: z.number(),
    total_revenue: z.number(),
  })
  .loose() satisfies z.ZodType<AdminComplexDetailResponse>;
