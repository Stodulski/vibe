import { useState } from 'react';
import { Link } from 'react-router-dom';
import { ChevronDown, ChevronRight } from 'lucide-react';
import { Button } from '@/shared/components/ui/button';
import { ES_AR } from '@/shared/i18n/es_AR';
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
  phone: '/settings',
  pricedCourt: '/courts',
  mercadoPago: '/settings?tab=billing',
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
    // The action sits beside what it fixes from `sm` up, so the step reads as
    // one row instead of a label with a button dangling under it.
    <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between sm:gap-4">
      <div className="min-w-0">
        <p className="text-text-primary text-sm font-medium">{copy.label}</p>
        <p className="text-text-secondary text-sm">{copy.why}</p>
      </div>
      <Button asChild size="sm" variant="outline" className="w-full shrink-0 sm:w-fit">
        <Link to={FIX_TARGET[id]}>{copy.action}</Link>
      </Button>
    </div>
  );
}

/** Title, "N de M listos" and the bar that shows it. */
function ProgressHeader({ done, total }: { done: number; total: number }) {
  const percent = Math.round((done / total) * 100);
  return (
    <div className="flex flex-col gap-2">
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
        className="bg-bg-highlight h-1.5 w-full overflow-hidden rounded-full"
      >
        <div className="bg-primary h-full rounded-full" style={{ width: `${String(percent)}%` }} />
      </div>
    </div>
  );
}

/** One compact row per pending item, the whole row a link to the screen that fixes it. */
function PendingList({ ids }: { ids: ProfileItemId[] }) {
  return (
    <ul id={LIST_ID} className="divide-border-subtle flex flex-col divide-y">
      {ids.map((id) => {
        const copy = t.dashboard.profileChecklist.items[id];
        return (
          <li key={id}>
            {/* The whole row is the link, not only the chevron: a 16px arrow
                is a hard target, and the label is what people aim at. */}
            <Link
              to={FIX_TARGET[id]}
              aria-label={copy.action}
              className="group flex items-center justify-between gap-3 py-2"
            >
              <span className="text-text-primary text-sm group-hover:underline group-hover:underline-offset-4">
                {copy.label}
              </span>
              <ChevronRight
                aria-hidden="true"
                className="text-text-tertiary group-hover:text-text-primary size-4 shrink-0 transition-colors"
              />
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
  const { data: courts } = useCourts(complex.id);
  const [expanded, setExpanded] = useState(readExpanded);
  if (!courts) return null;

  const items = buildProfileChecklist({ complex, courts });
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
    // A card like the dashboard's others (owner, 2026-11-11: the loose block
    // looked unfinished). Inside, spacing groups it: header and bar 8px apart,
    // 16px to the next step, and the disclosure set off below a rule.
    <section
      className="border-border-subtle bg-bg-subtle flex flex-col gap-4 rounded-2xl border p-4 sm:p-5"
      aria-labelledby="profile-checklist-title"
    >
      <ProgressHeader done={items.length - pending.length} total={items.length} />

      {!showList && nextId && <NextItem id={nextId} />}

      {hasMany && (
        <div className="border-border-subtle -mb-2 border-t pt-2">
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
        </div>
      )}

      {showList && <PendingList ids={pending} />}
    </section>
  );
}
