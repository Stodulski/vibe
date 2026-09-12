import { Building2, LayoutGrid } from 'lucide-react';
import { cn } from '@/shared/lib/utils';
import { ES_AR } from '@/shared/i18n/es_AR';
import type { Complex } from '@/shared/types/api.types';

const t = ES_AR;

interface ComplexCardProps {
  complex: Complex;
  onSelect: (complex: Complex) => void;
}

/**
 * One club, on the screen that asks which one to work on.
 *
 * It used to carry a "MercadoPago pendiente" warning. Two reasons it went:
 * taking cash is a legitimate choice — the wizard offers "Lo hago después" —
 * so the badge nagged about a decision, and a warning nobody can resolve or
 * dismiss teaches people to ignore warnings that matter. And this is a
 * SELECTOR: the only question it answers is which club to work on, and how a
 * club collects money does not change that answer.
 */
export function ComplexCard({ complex, onSelect }: ComplexCardProps) {
  return (
    <button
      onClick={() => {
        onSelect(complex);
      }}
      className={cn(
        'group flex flex-col rounded-2xl border border-border-subtle bg-bg-elevated p-4 text-left transition-colors duration-150 sm:p-5',
        'hover:border-primary-500/30 hover:bg-bg-elevated/80 hover:shadow-sm',
        'press-scale',
      )}
    >
      <div className="flex items-start gap-3">
        <div className="flex size-10 shrink-0 items-center justify-center rounded-xl bg-primary-500/10">
          <Building2 className="size-5 text-primary-500" />
        </div>
        <div className="min-w-0 flex-1">
          <p className="truncate text-sm font-semibold text-text-primary">{complex.name}</p>
          <p className="mt-0.5 truncate text-xs text-text-tertiary">
            {complex.city}, {complex.province}
          </p>
          {/* A state, not an alarm. The chip this replaces used an alert icon
              and warning colour for something the owner had chosen, and a
              warning nobody can resolve or dismiss teaches people to ignore
              warnings. This one is a fact: the club has not opened yet.

              Only rendered when the server counted — `court_count` is absent
              wherever it did not, and a missing count must not read as zero. */}
          {complex.court_count === 0 && (
            <p className="mt-1.5 flex items-center gap-1.5 text-xs text-text-secondary">
              <LayoutGrid className="size-3.5 shrink-0 text-text-tertiary" />
              {t.complex.needsCourts}
            </p>
          )}
        </div>
      </div>
    </button>
  );
}
