import type { MPFeesGroup } from '@/shared/schemas/mpFees.schema';

// `complexes.province` isn't a closed vocabulary: it's whatever the backend's
// Google Places proxy (`places/details`) hands back for `administrative_area_level_1`,
// and existing fixtures already disagree with each other for the same city —
// 'Buenos Aires', 'CABA', and 'Ciudad Autónoma de Buenos Aires' all show up as the
// province of a Buenos Aires City address across this repo's own test data
// (see `addressApi.test.ts` vs `address-input-test-helpers.tsx`). Every alias below
// collapses to the same canonical group name the landing's JSON is expected to use.
const CABA_ALIASES = new Set(['caba', 'capital federal', 'ciudad autonoma de buenos aires', 'ciudad de buenos aires']);
const CABA_CANONICAL = 'ciudad de buenos aires';

function normalizeProvince(raw: string): string {
  const withoutAccents = raw
    .normalize('NFD')
    .replace(/[\u0300-\u036f]/g, '')
    .toLowerCase();
  const withoutQualifiers = withoutAccents.replace(/\bprovincia de\b/g, '').replace(/\bprovince\b/g, '');
  const collapsed = withoutQualifiers.trim().replace(/\s+/g, ' ');
  return CABA_ALIASES.has(collapsed) ? CABA_CANONICAL : collapsed;
}

/** Finds the fee group whose `provincias` list contains `province`, ignoring accents, case, and "provincia de"/"province" qualifiers. */
export function matchMPFeesGroup(province: string, grupos: MPFeesGroup[]): MPFeesGroup | undefined {
  const target = normalizeProvince(province);
  return grupos.find((grupo) => grupo.provincias.some((candidate) => normalizeProvince(candidate) === target));
}
