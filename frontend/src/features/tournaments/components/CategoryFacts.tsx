import { useId, useState } from 'react';
import { ChevronDown } from 'lucide-react';
import { Button } from '@/shared/components/ui/button';
import { ES_AR } from '@/shared/i18n/es_AR';
import { categoryStartLabel } from '@/shared/lib/traitLabels';
import { cn, formatPrice } from '@/shared/lib/utils';
import type { CourtFeeSummary } from '../lib/courtFee';
import type { TournamentOwnerCategory } from '@/shared/types/api.types';

const t = ES_AR.tournaments;

/**
 * The category in one line of key facts: pairs taken, price per pair, format and
 * start. The rules (zones, sets and games, court fee) wait behind "Ver reglas",
 * closed until the owner asks for them.
 */
export function CategoryFacts({
  category,
  courtFee,
}: {
  category: TournamentOwnerCategory;
  courtFee: CourtFeeSummary;
}) {
  const [rulesOpen, setRulesOpen] = useState(false);
  const rulesId = useId();
  const facts = [
    t.detail.pairsRatio(category.confirmed_count, category.capacity),
    t.pricePerPair(formatPrice(category.price)),
    t.format[category.format],
    categoryStartLabel(category.starts_on, category.starts_at),
  ].filter((fact): fact is string => !!fact);

  return (
    <div className="min-w-0 flex-1 space-y-2">
      <p className="text-text-secondary text-sm">{facts.join(' · ')}</p>
      {category.prizes && <p className="text-text-tertiary text-xs">{category.prizes}</p>}
      <Button
        type="button"
        variant="ghost"
        size="sm"
        className="text-primary-400 hover:text-primary-300 h-auto gap-1.5 px-0 text-xs"
        aria-expanded={rulesOpen}
        aria-controls={rulesOpen ? rulesId : undefined}
        onClick={() => {
          setRulesOpen((open) => !open);
        }}
      >
        {rulesOpen ? t.detail.hideRules : t.detail.rules}
        <ChevronDown
          className={cn('size-3.5 transition-transform duration-200', rulesOpen && 'rotate-180')}
          aria-hidden="true"
        />
      </Button>
      {rulesOpen && (
        <div id={rulesId} className="text-text-secondary space-y-1 text-sm">
          {category.format === 'groups' && (
            <p>{t.detail.groupsSetup(category.group_size, category.qualifiers_per_group)}</p>
          )}
          <p>{t.detail.matchSetup(category.sets_to_win, category.games_per_set, category.super_tiebreak_decider)}</p>
          {category.court_fee > 0 && <p className="tabular-nums">{t.courtFee.line(formatPrice(category.court_fee))}</p>}
          {(courtFee.collected > 0 || courtFee.pending > 0) && (
            <p className="tabular-nums">
              {t.courtFee.summary(formatPrice(courtFee.collected), formatPrice(courtFee.pending))}
            </p>
          )}
        </div>
      )}
    </div>
  );
}
