import { useId } from 'react';
import { Button } from '@/shared/components/ui/button';
import { ES_AR } from '@/shared/i18n/es_AR';

interface DangerZoneProps {
  /** One short line saying what the action does and that it cannot be undone. */
  description: string;
  /** The destructive action's own words, e.g. "Eliminar complejo". */
  actionLabel: string;
  /** Opens the confirmation: nothing is deleted from here. */
  onAction: () => void;
}

/**
 * The last section of a page that owns an irreversible action: a title, one
 * line and a visible destructive outline button that opens the existing
 * confirmation. Never an overflow menu, which hid the only action it held.
 */
export function DangerZone({ description, actionLabel, onAction }: DangerZoneProps) {
  const titleId = useId();
  return (
    <section aria-labelledby={titleId} className="border-border-subtle mx-auto max-w-xl space-y-3 border-t pt-6">
      <div className="space-y-1">
        <h2 id={titleId} className="text-text-primary text-base font-semibold">
          {ES_AR.common.dangerZone}
        </h2>
        <p className="text-text-secondary text-sm">{description}</p>
      </div>
      <Button type="button" variant="destructive-outline" onClick={onAction}>
        {actionLabel}
      </Button>
    </section>
  );
}
