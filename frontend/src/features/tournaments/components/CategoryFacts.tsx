import { useId, useState } from 'react';
import { ChevronDown } from 'lucide-react';
import { Button } from '@/shared/components/ui/button';
import { ES_AR } from '@/shared/i18n/es_AR';
import { categoryStartLabel } from '@/shared/lib/traitLabels';
import { cn, formatPrice } from '@/shared/lib/utils';
import type { CourtFeeSummary } from '../lib/courtFee';
import type { TournamentOwnerCategory } from '@/shared/types/api.types';

const t = ES_AR.tournaments;

/** Joins the facts that are there: a fact with no value (no start date yet) is left out. */
function factsLine(facts: (string | null)[]): string {
  return facts.filter((fact): fact is string => !!fact).join(' · ');
}

/** The rules behind "Ver reglas", one line each: zones, sets and games, the court fee and what it has collected. */
function categoryRules(category: TournamentOwnerCategory, courtFee: CourtFeeSummary): string[] {
  return [
    category.format === 'groups' ? t.detail.groupsSetup(category.group_size, category.qualifiers_per_group) : null,
    t.detail.matchSetup(category.sets_to_win, category.games_per_set, category.super_tiebreak_decider),
    category.court_fee > 0 ? t.courtFee.line(formatPrice(category.court_fee)) : null,
    courtFee.collected > 0 || courtFee.pending > 0
      ? t.courtFee.summary(formatPrice(courtFee.collected), formatPrice(courtFee.pending))
      : null,
  ].filter((rule): rule is string => !!rule);
}

/**
 * The category in one line of key facts. From `sm` up it reads in full: pairs taken,
 * price per pair, format and start. A phone reads it short, "8/8 · $30.000 · Zonas",
 * without the start, so the line fits on one row at 256px. The rules (zones, sets and
 * games, court fee) wait behind "Ver reglas", closed until the owner asks for them.
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
  const pairs = t.detail.pairsRatio(category.confirmed_count, category.capacity);
  const pairsShort = t.detail.pairsRatioShort(category.confirmed_count, category.capacity);
  const price = formatPrice(category.price);
  const format = t.format[category.format];
  const start = categoryStartLabel(category.starts_on, category.starts_at);
  const summaryFull = factsLine([pairs, t.pricePerPair(price), format, start]);
  const summaryShort = factsLine([pairsShort, price, t.formatShort[category.format]]);
  const rules = categoryRules(category, courtFee);

  return (
    <div className="min-w-0 flex-1 space-y-2">
      <p className="text-text-secondary truncate text-sm" title={summaryFull}>
        <span className="sm:hidden">{summaryShort}</span>
        <span className="hidden sm:inline">{summaryFull}</span>
      </p>
      {category.prizes && (
        <p className="text-text-tertiary truncate text-xs" title={category.prizes}>
          {category.prizes}
        </p>
      )}
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
        <div id={rulesId} className="text-text-tertiary min-w-0 space-y-1 text-xs">
          {rules.map((rule) => (
            <p key={rule}>{rule}</p>
          ))}
        </div>
      )}
    </div>
  );
}
