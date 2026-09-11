import { useMemo } from 'react';
import type { PublicComplexResponse, Sport } from '@/shared/types/api.types';

export function useAvailableSports(complexData: PublicComplexResponse | undefined): Sport[] {
  return useMemo(() => {
    if (!complexData) return [];
    return [...new Set(complexData.courts.filter((c) => c.is_active).map((c) => c.sport))];
  }, [complexData]);
}
