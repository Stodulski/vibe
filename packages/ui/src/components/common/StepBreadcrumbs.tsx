import { Pencil } from 'lucide-react';

export interface Crumb<Id extends string = string> {
  id: Id;
  label: string;
  answer: string;
}

interface StepBreadcrumbsProps<Id extends string> {
  crumbs: Crumb<Id>[];
  onEdit: (step: Id) => void;
}

/**
 * The questions already answered, each one a way back into itself.
 *
 * Every crumb shows the ANSWER, never the name of the step: "90 min", not
 * "Duración". A crumb that only names the question is a progress bar wearing
 * a disguise — you cannot review what you cannot see, and the point of
 * keeping these on screen is that they are reviewable. The step's name is
 * kept for screen readers only: "Pádel", "90 min" and "09:30" are obvious to
 * the eye but not to someone hearing a list of buttons.
 *
 * They replace a back button, which would only ever undo one step: correcting
 * the first answer from the last question would cost as many taps as there
 * are steps between them. A crumb costs one, from anywhere. Chapter 8 (p.361)
 * asks a stepped form for two things — show the progress, and let people
 * revisit and change their answers. A back button does neither.
 *
 * Shared because two levels of the booking flow use it: the page's sport and
 * duration steps, and the selector's own hour step once a court question is
 * open.
 */
export function StepBreadcrumbs<Id extends string>({ crumbs, onEdit }: StepBreadcrumbsProps<Id>) {
  if (crumbs.length === 0) return null;

  return (
    <div className="flex flex-wrap gap-2">
      {crumbs.map((crumb) => (
        <button
          key={crumb.id}
          type="button"
          onClick={() => {
            onEdit(crumb.id);
          }}
          className="border-border-subtle bg-bg-subtle hover:border-border-default hover:bg-bg-overlay flex items-center gap-1.5 rounded-full border px-3 py-1.5 text-xs transition-colors"
        >
          <span className="sr-only">{crumb.label}</span>
          <span className="text-text-primary font-semibold">{crumb.answer}</span>
          <Pencil className="text-primary-400 size-3 shrink-0" aria-hidden="true" />
        </button>
      ))}
    </div>
  );
}
