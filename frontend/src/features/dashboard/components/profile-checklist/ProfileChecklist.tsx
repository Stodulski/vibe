import { useState } from 'react';
import { Link } from 'react-router-dom';
import { ChevronDown, ChevronRight } from 'lucide-react';
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

const LIST_ID = 'profile-checklist-pending';
const EXPANDED_KEY = 'vibe.profileChecklist.expanded';

/** Remembered per browser. Storage can be blocked (private mode, policy), and then the choice just resets. */
function readExpanded(): boolean {
  try {
    return window.localStorage.getItem(EXPANDED_KEY) === '1';
  } catch {
    return false;
  }
}

function writeExpanded(expanded: boolean) {
  try {
    window.localStorage.setItem(EXPANDED_KEY, expanded ? '1' : '0');
  } catch {
    // Not remembered; the card still works.
  }
}

/** The single next step: its label, why it matters, and the one action that fixes it. */
function NextItem({ id }: { id: ProfileItemId }) {
  const copy = t.dashboard.profileChecklist.items[id];
  return (
    <div className="flex flex-col gap-2">
      <div className="min-w-0">
        <p className="text-text-primary text-sm font-medium">{copy.label}</p>
        <p className="text-text-tertiary text-sm">{copy.why}</p>
      </div>
      <Button asChild size="sm" variant="outline" className="w-full sm:w-auto">
        <Link to={FIX_TARGET[id]}>{copy.action}</Link>
      </Button>
    </div>
  );
}

/** Title, "N de 8 listos" and the bar that shows it. */
function ProgressHeader({ done, total }: { done: number; total: number }) {
  const percent = Math.round((done / total) * 100);
  return (
    <>
      <div className="flex items-baseline justify-between gap-2">
        <h3 id="profile-checklist-title" className="text-text-primary text-sm font-semibold">
          {t.dashboard.profileChecklist.title}
        </h3>
        <span className="text-text-tertiary shrink-0 text-sm">
          {t.dashboard.profileChecklist.progress(done, total)}
        </span>
      </div>
      <div
        role="progressbar"
        aria-label={t.dashboard.profileChecklist.title}
        aria-valuemin={0}
        aria-valuemax={total}
        aria-valuenow={done}
        className="bg-bg-subtle h-1.5 w-full overflow-hidden rounded-full"
      >
        <div className="bg-primary h-full rounded-full" style={{ width: `${String(percent)}%` }} />
      </div>
    </>
  );
}

/** One compact row per pending item: its label, and a chevron to the screen that fixes it. */
function PendingList({ ids }: { ids: ProfileItemId[] }) {
  return (
    <ul id={LIST_ID} className="divide-border-subtle flex flex-col divide-y">
      {ids.map((id) => {
        const copy = t.dashboard.profileChecklist.items[id];
        return (
          <li key={id} className="flex items-center justify-between gap-3 py-2">
            <span className="text-text-primary text-sm">{copy.label}</span>
            <Link
              to={FIX_TARGET[id]}
              aria-label={copy.action}
              className="text-text-tertiary hover:text-text-primary shrink-0"
            >
              <ChevronRight aria-hidden="true" className="size-4" />
            </Link>
          </li>
        );
      })}
    </ul>
  );
}

interface ProfileChecklistProps {
  complex: Complex;
}

/**
 * What the public page of this complex is still missing. Collapsed it shows
 * only the next pending step, so the operational cards below stay in reach;
 * expanded it lists every pending item as a compact row. Shows nothing while
 * the data loads, and nothing once the profile is complete.
 */
export function ProfileChecklist({ complex }: ProfileChecklistProps) {
  const { data: schedules } = useSchedules(complex.id, complex.slug);
  const { data: courts } = useCourts(complex.id);
  const [expanded, setExpanded] = useState(readExpanded);
  if (!schedules || !courts) return null;

  const items = buildProfileChecklist({ complex, schedules, courts });
  const pending = items.filter((item) => !item.done).map((item) => item.id);
  if (pending.length === 0) return null;

  const [nextId] = pending;
  // A single pending item is already the next step, so a disclosure would only repeat it.
  const hasMany = pending.length > 1;
  const showList = expanded && hasMany;

  const toggle = () => {
    const next = !expanded;
    setExpanded(next);
    writeExpanded(next);
  };

  return (
    <Panel as="section" size="sm" className="flex flex-col gap-2 px-4 py-3" aria-labelledby="profile-checklist-title">
      <ProgressHeader done={items.length - pending.length} total={items.length} />

      {!showList && nextId && <NextItem id={nextId} />}

      {hasMany && (
        <Button
          type="button"
          variant="ghost"
          size="sm"
          className="-ml-2 w-fit"
          aria-expanded={expanded}
          aria-controls={LIST_ID}
          onClick={toggle}
        >
          {expanded
            ? t.dashboard.profileChecklist.hidePending
            : t.dashboard.profileChecklist.showPending(pending.length)}
          <ChevronDown aria-hidden="true" className={`size-4 transition-transform ${expanded ? 'rotate-180' : ''}`} />
        </Button>
      )}

      {showList && <PendingList ids={pending} />}
    </Panel>
  );
}
