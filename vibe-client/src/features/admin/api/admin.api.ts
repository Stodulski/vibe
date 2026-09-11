import api, { withSignal } from '@/shared/lib/ky';
import type {
  PlatformStatsResponse,
  AdminUsersResponse,
  AdminUserDetailResponse,
  AdminComplexesResponse,
  AdminComplexDetailResponse,
} from '@/shared/types/api.types';
import { parseWith } from '@/shared/lib/apiParse';
import { messageResponseSchema } from '@/shared/schemas/envelope.schema';
import {
  platformStatsResponseSchema,
  adminUsersResponseSchema,
  adminUserDetailResponseSchema,
  adminComplexesResponseSchema,
  adminComplexDetailResponseSchema,
} from '@/shared/schemas/admin.schema';

export const adminApi = {
  getStats: (signal?: AbortSignal): Promise<PlatformStatsResponse> =>
    api.get('admin/stats', withSignal(signal)).json().then(parseWith(platformStatsResponseSchema, 'adminApi.getStats')),

  getUsers: (
    params: {
      search?: string | undefined;
      role?: string | undefined;
      cursor?: string | undefined;
      limit?: number;
    },
    signal?: AbortSignal,
  ): Promise<AdminUsersResponse> => {
    const searchParams: Record<string, string> = {};
    if (params.search) searchParams.search = params.search;
    if (params.role) searchParams.role = params.role;
    if (params.cursor) searchParams.cursor = params.cursor;
    if (params.limit) searchParams.limit = String(params.limit);
    return api
      .get('admin/users', { searchParams, ...withSignal(signal) })
      .json()
      .then(parseWith(adminUsersResponseSchema, 'adminApi.getUsers'));
  },

  getUserDetail: (userId: string, signal?: AbortSignal): Promise<AdminUserDetailResponse> =>
    api
      .get(`admin/users/${userId}`, withSignal(signal))
      .json()
      .then(parseWith(adminUserDetailResponseSchema, 'adminApi.getUserDetail')),

  getComplexes: (
    params: { search?: string | undefined; cursor?: string | undefined; limit?: number },
    signal?: AbortSignal,
  ): Promise<AdminComplexesResponse> => {
    const searchParams: Record<string, string> = {};
    if (params.search) searchParams.search = params.search;
    if (params.cursor) searchParams.cursor = params.cursor;
    if (params.limit) searchParams.limit = String(params.limit);
    return api
      .get('admin/complexes', { searchParams, ...withSignal(signal) })
      .json()
      .then(parseWith(adminComplexesResponseSchema, 'adminApi.getComplexes'));
  },

  getComplexDetail: (complexId: string, signal?: AbortSignal): Promise<AdminComplexDetailResponse> =>
    api
      .get(`admin/complexes/${complexId}`, withSignal(signal))
      .json()
      .then(parseWith(adminComplexDetailResponseSchema, 'adminApi.getComplexDetail')),

  toggleUserActive: (userId: string, isActive: boolean): Promise<{ message: string }> =>
    api
      .patch(`admin/users/${userId}/toggle-active`, { json: { is_active: isActive } })
      .json()
      .then(parseWith(messageResponseSchema, 'adminApi.toggleUserActive')),
};
