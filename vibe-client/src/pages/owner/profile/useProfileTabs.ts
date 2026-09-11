import { useSearchParams } from 'react-router-dom';
import { TABS, type TabValue } from './tabs';

function resolveTab(param: string | null): TabValue {
  return param && TABS.some((tab) => tab.value === param) ? (param as TabValue) : 'personal';
}

/**
 * The active tab is derived from `?tab=` on every render rather than copied
 * into its own `useState` — a `useState(initialTab)` only reads the URL
 * once, so the browser's back/forward buttons (which change `searchParams`
 * without going through `handleTabChange`) would leave the tab stuck on
 * whatever it was when the component first mounted.
 */
export function useProfileTabs() {
  const [searchParams, setSearchParams] = useSearchParams();
  const activeTab = resolveTab(searchParams.get('tab'));

  const handleTabChange = (tab: TabValue) => {
    setSearchParams(tab === 'personal' ? {} : { tab }, { replace: true });
  };

  return { activeTab, handleTabChange };
}
