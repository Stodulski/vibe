import { usePageTitle } from '@/shared/hooks/usePageTitle';
import { PageHeader } from '@/shared/components/common/PageHeader';
import { PersonalInfoForm, SecurityForm } from '@/features/auth';
import { ES_AR } from '@/shared/i18n/es_AR';
import { SectionNav } from '@/shared/components/common/section-nav/SectionNav';
import { SectionTabs } from '@/shared/components/common/section-nav/SectionTabs';
import { TABS } from './profile/tabs';
import { useProfileTabs } from './profile/useProfileTabs';

const t = ES_AR;

export default function ProfilePage() {
  usePageTitle(t.navigation.profile);
  const { activeTab, handleTabChange } = useProfileTabs();

  return (
    <div className="animate-fade-in">
      <PageHeader title={t.navigation.profile} />

      <div>
        <div className="flex flex-col gap-3 sm:gap-4 lg:flex-row">
          <SectionNav items={TABS} active={activeTab} onChange={handleTabChange} />
          <SectionTabs items={TABS} active={activeTab} onChange={handleTabChange} />

          {/* Content */}
          {/* A card from lg, bare below it — the same treatment settings has.
              On a phone the border sat just inside the screen's own edge around
              content that fills the width anyway. */}
          <div className="min-w-0 flex-1 lg:max-w-[39rem] lg:rounded-2xl lg:border lg:border-border-subtle lg:bg-bg-subtle lg:p-6">
            {activeTab === 'personal' && <PersonalInfoForm />}
            {activeTab === 'security' && <SecurityForm />}
          </div>
        </div>
      </div>
    </div>
  );
}
