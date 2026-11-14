import { useState } from 'react';
import { CheckCircle2 } from 'lucide-react';
import { Button, formatPrice } from '@vibe/ui';
import { BOOKING_MESSAGES } from '../../messages';
import type { PublicTournamentDetail, PublicTournamentCategory } from '../../api/types';
import { BracketView } from './BracketView';
import { GroupSection } from './GroupSection';
import { PairSignUpForm } from './PairSignUpForm';
import { ScheduleList } from './ScheduleList';
import { formatLabel, isOpenForSignUp, matchFormatLabel, spotsLeftLabel } from './tournamentModel';

const t = BOOKING_MESSAGES.tournaments;

function Notice({ children }: { children: string }) {
  return (
    <p role="status" className="text-text-secondary text-sm">
      <CheckCircle2 className="mr-1 inline size-4 align-text-bottom" aria-hidden="true" />
      {children}
    </p>
  );
}

function noticeFor(category: PublicTournamentCategory): string {
  if (category.status === 'cancelled') return t.categoryCancelled;
  if (category.status === 'closed' || category.draw_generated) return t.signUpClosedCategory;
  return t.categoryFull;
}

interface SignUpAreaProps {
  slug: string;
  tournamentId: string;
  category: PublicTournamentCategory;
  tournamentActive: boolean;
}

/** The open category offers its form on demand; any other says why it does not. */
function SignUpArea({ slug, tournamentId, category, tournamentActive }: SignUpAreaProps) {
  const [formOpen, setFormOpen] = useState(false);
  if (!tournamentActive) return null;
  if (!isOpenForSignUp(category)) return <Notice>{noticeFor(category)}</Notice>;

  return (
    <div className="flex flex-col gap-4">
      {!formOpen && (
        <Button
          size="lg"
          className="min-h-12 w-full sm:w-auto"
          onClick={() => {
            setFormOpen(true);
          }}
        >
          {t.signUp}
        </Button>
      )}
      {formOpen && <PairSignUpForm slug={slug} tournamentId={tournamentId} category={category} />}
    </div>
  );
}

interface CategorySectionProps {
  slug: string;
  detail: PublicTournamentDetail;
  category: PublicTournamentCategory;
}

/** One category: its format, price and spots, the sign-up while open, and its groups, bracket and schedule once drawn. */
export function CategorySection({ slug, detail, category }: CategorySectionProps) {
  const { tournament, pairs, groups, matches } = detail;
  const headingId = `tournament-category-${category.id}`;
  const own = {
    pairs: pairs.filter((p) => p.category_id === category.id),
    groups: groups.filter((g) => g.category_id === category.id).sort((a, b) => a.position - b.position),
    matches: matches.filter((m) => m.category_id === category.id),
  };
  const groupNames = new Map(groups.map((g) => [g.id, g.name]));
  const open = isOpenForSignUp(category) && tournament.status === 'active';
  const hasBracket = own.matches.some((m) => m.stage === 'bracket');
  const hasSchedule = own.matches.some((m) => m.schedule !== null && !m.bye);

  return (
    <section
      aria-labelledby={headingId}
      className="border-border-subtle bg-bg-subtle flex min-w-0 flex-col gap-5 rounded-2xl border p-4 sm:p-5"
    >
      <div className="flex flex-col gap-1">
        <h2 id={headingId} className="text-text-primary text-lg font-bold">
          {category.name}
        </h2>
        <p className="text-text-secondary text-sm">{formatLabel(category.format)}</p>
        <p className="text-text-secondary text-sm">{matchFormatLabel(category)}</p>
        <div className="flex flex-wrap items-baseline justify-between gap-x-4 gap-y-1 pt-1">
          <p className="text-text-primary text-base font-semibold tabular-nums">
            {formatPrice(category.total_price ?? category.price)}{' '}
            <span className="text-text-secondary text-sm font-normal">{t.perPair}</span>
          </p>
          {open && <p className="text-primary-400 text-sm font-medium">{spotsLeftLabel(category.spots_left)}</p>}
        </div>
      </div>

      <SignUpArea
        slug={slug}
        tournamentId={tournament.id}
        category={category}
        tournamentActive={tournament.status === 'active'}
      />

      {own.groups.length > 0 && (
        <div className="flex flex-col gap-5">
          <h3 className="text-text-primary text-base font-bold">{t.groupsTitle}</h3>
          {own.groups.map((group) => (
            <GroupSection key={group.id} group={group} category={category} pairs={own.pairs} matches={own.matches} />
          ))}
        </div>
      )}

      {hasBracket && (
        <div className="flex min-w-0 flex-col gap-3">
          <h3 className="text-text-primary text-base font-bold">{t.bracketTitle}</h3>
          <BracketView matches={own.matches} pairs={own.pairs} groupNames={groupNames} />
        </div>
      )}

      {hasSchedule && (
        <div className="flex flex-col gap-3">
          <h3 className="text-text-primary text-base font-bold">{t.scheduleTitle}</h3>
          <ScheduleList matches={own.matches} pairs={own.pairs} groupNames={groupNames} />
        </div>
      )}
    </section>
  );
}
