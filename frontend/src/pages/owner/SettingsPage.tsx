import { AlertTriangle } from 'lucide-react';
import { usePageTitle } from '@/shared/hooks/usePageTitle';
import { PageHeader } from '@/shared/components/common/PageHeader';
import { EmptyState } from '@/shared/components/common/EmptyState';
// `useSelectedComplex` doesn't surface the underlying query's error/refetch
// (it's outside this fix's allowed paths — see the fix report). Reading
// `useComplexes` directly here, alongside it, is the smallest way to get
// `isError`/`refetch` without touching `features/complex`; TanStack Query
// dedupes both hooks' identical query key into a single request.
import { useSelectedComplex, useComplexes } from '@/features/complex';
import { SkeletonSettings } from '@/shared/components/common/Skeletons';
import { ES_AR } from '@/shared/i18n/es_AR';
import { SectionNav } from '@/shared/components/common/section-nav/SectionNav';
import { SectionTabs } from '@/shared/components/common/section-nav/SectionTabs';
import { TABS } from './settings/tabs';
import { SettingsTabContent } from './settings/SettingsTabContent';
import { useSettingsTabs } from './settings/useSettingsTabs';

const t = ES_AR;

export default function SettingsPage() {
  usePageTitle(t.complex.settings);
  const { complex, isLoading } = useSelectedComplex();
  const { isError, refetch } = useComplexes();
  const { activeTab, handleTabChange } = useSettingsTabs(complex);

  if (isError) {
    return (
      <div className="animate-fade-in">
        <EmptyState
          icon={AlertTriangle}
          title={t.common.loadError}
          description={t.common.loadErrorDescription}
          actionLabel={t.layout.retry}
          onAction={() => {
            void refetch();
          }}
        />
      </div>
    );
  }

  if (isLoading || !complex) {
    return (
      <div className="animate-fade-in">
        <SkeletonSettings />
      </div>
    );
  }

  return (
    <div className="animate-fade-in">
      <PageHeader title={t.complex.settings} />

      <div className="flex flex-col gap-3 sm:gap-4 lg:flex-row">
        <SectionNav items={TABS} active={activeTab} onChange={handleTabChange} />
        <SectionTabs items={TABS} active={activeTab} onChange={handleTabChange} />

        {/* 39rem = the 36rem content column plus the panel's own 1.5rem of
            padding on each side. Sized to the content exactly, so the gap is
            24px on all four edges; at max-w-2xl the centred column had 24px of
            slack left over and the card read as padded wider than it was tall. */}
        {/* A card from lg, bare below it. On a phone the card was a border
            drawn just inside another border — the screen edge — around content
            that fills the width anyway, so it spent 32px of a 320px screen
            saying nothing. On a wide screen it earns its keep: it is what marks
            the section off from the space beside it. */}
        <div className="min-w-0 flex-1 lg:max-w-[39rem] lg:rounded-2xl lg:border lg:border-border-subtle lg:bg-bg-subtle lg:p-6">
          <SettingsTabContent activeTab={activeTab} complex={complex} />
        </div>
      </div>
    </div>
  );
}
