import { ScheduleConfig } from '@/features/complex';
import type { Complex } from '@/shared/types/api.types';
import { SettingsGeneralTab } from './SettingsGeneralTab';
import { SettingsBillingTab } from './SettingsBillingTab';
import type { TabValue } from './tabs';

interface SettingsTabContentProps {
  activeTab: TabValue;
  complex: Complex;
}

export function SettingsTabContent({ activeTab, complex }: SettingsTabContentProps) {
  if (activeTab === 'general') return <SettingsGeneralTab complex={complex} />;
  if (activeTab === 'schedules') return <ScheduleConfig complexId={complex.id} slug={complex.slug} />;
  return <SettingsBillingTab complex={complex} />;
}
