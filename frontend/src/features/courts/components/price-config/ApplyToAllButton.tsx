import { Copy } from 'lucide-react';
import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;

interface ApplyToAllButtonProps {
  onClick: () => void;
}

/**
 * Copies this day's rate onto every day.
 *
 * Icon-only, and neutral: it is a convenience beside the field, not the
 * dialog's action, and it read as weekend-or-weekday coloured for no reason
 * other than the row it happened to sit in.
 *
 * `size-11` on touch so the square is 44px wide as well as tall — the app's
 * coarse-pointer floor in globals.css sets the height of every button but
 * cannot widen an icon button, which left this one 36px across on a phone.
 */
export function ApplyToAllButton({ onClick }: ApplyToAllButtonProps) {
  return (
    <button
      type="button"
      title={t.courts.applyToAll}
      aria-label={t.courts.applyToAll}
      onClick={onClick}
      className="text-text-secondary hover:bg-bg-highlight hover:text-text-primary flex size-11 shrink-0 items-center justify-center rounded-lg transition-colors sm:size-9"
    >
      <Copy className="size-3.5" />
    </button>
  );
}
