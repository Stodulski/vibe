import { PageHeaderSkeleton } from '@/shared/components/common/PageHeaderSkeleton';
import { SectionNavSkeleton } from '@/shared/components/common/section-nav/SectionNavSkeleton';
import { SectionTabsSkeleton } from '@/shared/components/common/section-nav/SectionTabsSkeleton';
import { SECTION_LAYOUT_CLASS, SECTION_PANEL_CLASS } from '@/shared/components/common/section-nav/sectionNavLayout';
import { ES_AR } from '@/shared/i18n/es_AR';
import { ComplexFormSkeleton } from '../components/ComplexFormSkeleton';
import { TABS } from './settings/tabs';

const t = ES_AR;

/**
 * What `SettingsPage` draws while its route chunk loads: the mobile-only
 * title, the section list (from `lg`) or tab strip (below it), and the
 * content panel with the General tab's form.
 */
export function SettingsPageSkeleton() {
  return (
    <div role="status" aria-label={t.common.loading}>
      <PageHeaderSkeleton titleWidth="w-36" />
      <div className={SECTION_LAYOUT_CLASS}>
        <SectionNavSkeleton count={TABS.length} />
        <SectionTabsSkeleton count={TABS.length} />
        <div className={SECTION_PANEL_CLASS}>
          <ComplexFormSkeleton />
        </div>
      </div>
    </div>
  );
}
