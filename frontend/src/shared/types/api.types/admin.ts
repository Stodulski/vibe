import type { Complex } from './complex';
import type { Ok, Spec } from './spec';

// ─── Admin ───

export type PlatformStats = Spec<'PlatformStats'>;

export type PlatformStatsResponse = Ok<'adminGetStats'>;

export type AdminUserRow = Spec<'AdminUserRow'>;

export type AdminUsersResponse = Ok<'adminListUsers'>;

// `complexes` is remapped to the narrowed `Complex` (closed `amenities`
// union); the document's own schema types that field as `string[]`.
export type AdminUserDetailResponse = Omit<Ok<'adminGetUser'>, 'complexes'> & { complexes: Complex[] };

export type AdminComplexRow = Spec<'AdminComplexRow'>;

export type AdminComplexesResponse = Ok<'adminListComplexes'>;

export type AdminComplexDetailResponse = Omit<Ok<'adminGetComplex'>, 'complex'> & { complex: Complex };
