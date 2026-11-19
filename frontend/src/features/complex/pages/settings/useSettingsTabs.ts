import { useEffect } from 'react';
import { useSearchParams } from 'react-router-dom';
import { noop, useQueryClient } from '@tanstack/react-query';
import { queryKeys } from '@/shared/lib/queryKeys';
import { complexApi } from '@/features/complex';
import { useInstagramStatus } from '../../hooks/useInstagram';
import { INSTAGRAM_TAB, TABS, type TabValue } from './tabs';
import type { Complex } from '@/shared/types/api.types';

/**
 * Where the five old sections went.
 *
 * A `?tab=` link the owner bookmarked or a teammate shared still names a
 * section that no longer exists. Without this it silently lands on General,
 * which is the wrong screen for three of the five and gives no clue why.
 */
const MERGED_INTO: Record<string, TabValue> = {
  images: 'general',
  deposit: 'billing',
  mercadopago: 'billing',
};

/**
 * `instagramEnabled` is undefined while the status loads: `?tab=instagram` (the
 * OAuth callback's landing) then stays put and the tab shows its skeleton,
 * instead of flashing General first. Only an answer of `enabled: false` sends
 * it to General.
 */
function resolveTab(param: string | null, instagramEnabled: boolean | undefined): TabValue {
  if (!param) return 'general';
  if (param === INSTAGRAM_TAB.value) return instagramEnabled === false ? 'general' : INSTAGRAM_TAB.value;
  if (TABS.some((tab) => tab.value === param)) return param as TabValue;
  return MERGED_INTO[param] ?? 'general';
}

// The active tab is derived from `?tab=` on every render rather than copied
// into its own `useState` — a `useState(initialTab)` only reads the URL
// once, so the browser's back/forward buttons (which change `searchParams`
// without going through `handleTabChange`) would leave the tab stuck on
// whatever it was when the component first mounted.
export function useSettingsTabs(complex: Complex | null | undefined) {
  const [searchParams, setSearchParams] = useSearchParams();
  const { data: instagram } = useInstagramStatus(complex?.id);
  const activeTab = resolveTab(searchParams.get('tab'), instagram?.enabled);
  const tabs = instagram?.enabled ? [...TABS, INSTAGRAM_TAB] : TABS;

  // Prefetch schedules so the tab loads instantly
  const queryClient = useQueryClient();
  useEffect(() => {
    if (complex?.slug) {
      queryClient
        .query({
          queryKey: queryKeys.complexes.schedules(complex.id),
          queryFn: ({ signal }) => complexApi.getPublicComplex(complex.slug, signal),
          staleTime: 10 * 60 * 1000,
        })
        .catch(noop);
    }
  }, [complex?.id, complex?.slug, queryClient]);

  const handleTabChange = (tab: TabValue) => {
    setSearchParams(tab === 'general' ? {} : { tab }, { replace: true });
  };

  return { tabs, activeTab, handleTabChange };
}
