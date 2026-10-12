import { Link } from 'react-router-dom';
import { Panel } from '@/shared/components/common/Panel';
import { Button } from '@/shared/components/ui/button';
import { ES_AR } from '@/shared/i18n/es_AR';
import { useSchedules } from '@/features/complex';
import { useCourts } from '@/features/courts';
import type { Complex } from '@/shared/types/api.types';
import { buildProfileChecklist, type ProfileItemId } from './profileChecklist';

const t = ES_AR;

/** The screen that edits each item. Photos, description, services, map and phone all live on the general tab. */
const FIX_TARGET: Record<ProfileItemId, string> = {
  cover: '/settings',
  logo: '/settings',
  description: '/settings',
  amenities: '/settings',
  location: '/settings',
  hours: '/settings?tab=schedules',
  phone: '/settings',
  pricedCourt: '/courts',
};

interface ProfileChecklistProps {
  complex: Complex;
}

/**
 * What the public page of this complex is still missing, with a link to the
 * exact place that fixes each gap. Shows nothing while the data loads, and
 * nothing once the profile is complete, so it never sits there as clutter.
 */
export function ProfileChecklist({ complex }: ProfileChecklistProps) {
  const { data: schedules } = useSchedules(complex.id, complex.slug);
  const { data: courts } = useCourts(complex.id);
  if (!schedules || !courts) return null;

  const items = buildProfileChecklist({ complex, schedules, courts });
  const missing = items.filter((item) => !item.done);
  if (missing.length === 0) return null;

  const doneCount = items.length - missing.length;
  const progress = t.dashboard.profileChecklist.progress(doneCount, items.length);
  const percent = Math.round((doneCount / items.length) * 100);

  return (
    <Panel as="section" size="sm" className="flex flex-col gap-3 p-4" aria-labelledby="profile-checklist-title">
      <div className="flex flex-col gap-2">
        <div className="flex items-baseline justify-between gap-2">
          <h3 id="profile-checklist-title" className="text-text-primary text-sm font-semibold">
            {t.dashboard.profileChecklist.title}
          </h3>
          <span className="text-text-tertiary shrink-0 text-sm">{progress}</span>
        </div>
        <div
          role="progressbar"
          aria-label={t.dashboard.profileChecklist.title}
          aria-valuemin={0}
          aria-valuemax={items.length}
          aria-valuenow={doneCount}
          className="bg-bg-subtle h-1.5 w-full overflow-hidden rounded-full"
        >
          <div className="bg-primary h-full rounded-full" style={{ width: `${String(percent)}%` }} />
        </div>
      </div>

      <p className="text-text-tertiary text-sm">{t.dashboard.profileChecklist.intro}</p>

      <ul className="space-y-3">
        {missing.map(({ id }) => {
          const copy = t.dashboard.profileChecklist.items[id];
          return (
            <li key={id} className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between sm:gap-4">
              <div className="min-w-0">
                <p className="text-text-primary text-sm font-medium">{copy.label}</p>
                <p className="text-text-tertiary text-sm">{copy.why}</p>
              </div>
              <Button asChild size="sm" variant="outline" className="w-full shrink-0 sm:w-auto">
                <Link to={FIX_TARGET[id]}>{copy.action}</Link>
              </Button>
            </li>
          );
        })}
      </ul>
    </Panel>
  );
}
