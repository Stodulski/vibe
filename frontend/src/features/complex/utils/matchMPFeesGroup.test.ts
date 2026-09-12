import { describe, it, expect } from 'vitest';
// @vitest-environment node
import { matchMPFeesGroup } from './matchMPFeesGroup';
import type { MPFeesGroup } from '@/shared/schemas/mpFees.schema';

const grupos: MPFeesGroup[] = [
  { provincias: ['Buenos Aires', 'Chubut', 'Entre Ríos'], tasas: [6.6, 4.61, 3.56, 1.56] },
  { provincias: ['Ciudad de Buenos Aires'], tasas: [6.5, 4.5, 3.5, 1.5] },
  { provincias: ['Córdoba', 'Santa Fe'], tasas: [6.2, 4.2, 3.2, 1.2] },
];

describe('matchMPFeesGroup', () => {
  it('matches an exact province name', () => {
    expect(matchMPFeesGroup('Buenos Aires', grupos)).toBe(grupos[0]);
  });

  it('matches "Provincia de Buenos Aires"', () => {
    expect(matchMPFeesGroup('Provincia de Buenos Aires', grupos)).toBe(grupos[0]);
  });

  it('matches "Santa Fe Province"', () => {
    expect(matchMPFeesGroup('Santa Fe Province', grupos)).toBe(grupos[2]);
  });

  it('matches accented province names case-insensitively', () => {
    expect(matchMPFeesGroup('entre RÍOS', grupos)).toBe(grupos[0]);
  });

  it.each(['CABA', 'Capital Federal', 'Ciudad Autónoma de Buenos Aires', 'Ciudad de Buenos Aires'])(
    'matches the CABA alias %s to the CABA group',
    (alias) => {
      expect(matchMPFeesGroup(alias, grupos)).toBe(grupos[1]);
    },
  );

  it('returns undefined for an unknown province', () => {
    expect(matchMPFeesGroup('Marte', grupos)).toBeUndefined();
  });
});
