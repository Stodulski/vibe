import { type LucideIcon } from 'lucide-react';
import { Button, formatPrice } from '@vibe/ui';
import { BOOKING_MESSAGES } from '../../messages';
import type { PublicTournament, PublicTournamentDetail, PublicTournamentCategory } from '../../api/types';
import { categoryTitle } from '../../lib/traitLabels';
import { publicTournamentSignUpPath } from '../../lib/publicPaths';
import { BracketView } from './BracketView';
import { noticeFor } from './signUpNotice';
import { GroupSection } from './GroupSection';
import { ScheduleList } from './ScheduleList';
import { categoryWhen } from './agendaModel';
import { formatLabel, isOpenForSignUp, matchFormatLabel, spotsLeftLabel } from './tournamentModel';

const t = BOOKING_MESSAGES.tournaments;

export function Notice({ icon: Icon, children }: { icon: LucideIcon; children: string }) {
  return (
    <p role="status" className="text-text-secondary text-sm">
      <Icon className="mr-1 inline size-4 align-text-bottom" aria-hidden="true" />
      {children}
    </p>
  );
}

interface SignUpAreaProps {
  slug: string;
  tournament: PublicTournament;
  category: PublicTournamentCategory;
  tournamentActive: boolean;
}

/** The open category links to its own sign-up page; any other says why it does not. */
function SignUpArea({ slug, tournament, category, tournamentActive }: SignUpAreaProps) {
  if (!tournamentActive) return null;
  if (!isOpenForSignUp(category)) {
    const notice = noticeFor(category);
    return <Notice icon={notice.icon}>{notice.text}</Notice>;
  }

  return (
    <Button asChild size="lg" className="min-h-12 w-full sm:w-auto sm:self-start">
      <a href={publicTournamentSignUpPath(slug, tournament.id, category.id)}>{t.signUp}</a>
    </Button>
  );
}

interface CategoryHeaderProps {
  headingId: string;
  category: PublicTournamentCategory;
  /** The tournament's sport: it decides the words of the category's title. */
  sport: string;
  /** The category takes pairs now: the spots left are worth saying. */
  showOffer: boolean;
}

/**
 * The category's title, its day and prizes when it has them, its format and,
 * unless it was cancelled, how it is played, its price and its spots.
 */
function CategoryHeader({ headingId, category, sport, showOffer }: CategoryHeaderProps) {
  const cancelled = category.status === 'cancelled';
  return (
    <div className="flex flex-col gap-1">
      <h2 id={headingId} className="text-text-primary text-lg font-bold">
        {categoryTitle(category, sport)}
      </h2>
      {category.starts_on && (
        <p className="text-text-secondary text-sm first-letter:uppercase">
          {categoryWhen({ date: category.starts_on, time: category.starts_at })}
        </p>
      )}
      {category.prizes && (
        <p className="text-text-secondary text-sm">
          <span className="text-text-tertiary">{`${t.prizesLabel}: `}</span>
          {category.prizes}
        </p>
      )}
      <p className="text-text-secondary text-sm">{formatLabel(category.format)}</p>
      {!cancelled && (
        <>
          <p className="text-text-secondary text-sm">{matchFormatLabel(category)}</p>
          <div className="flex flex-wrap items-baseline justify-between gap-x-4 gap-y-1 pt-1">
            <p className="text-text-primary text-base font-semibold tabular-nums">
              {formatPrice(category.price)}{' '}
              <span className="text-text-secondary text-sm font-normal">{t.perPair}</span>
            </p>
            {showOffer && <p className="text-primary-400 text-sm font-medium">{spotsLeftLabel(category.spots_left)}</p>}
          </div>
        </>
      )}
    </div>
  );
}

interface CategorySectionProps {
  slug: string;
  detail: PublicTournamentDetail;
  category: PublicTournamentCategory;
}

/** The pairs, groups and matches of one category, out of the tournament's whole detail. */
function sliceOf(detail: PublicTournamentDetail, categoryId: string) {
  return {
    pairs: detail.pairs.filter((p) => p.category_id === categoryId),
    groups: detail.groups.filter((g) => g.category_id === categoryId).sort((a, b) => a.position - b.position),
    matches: detail.matches.filter((m) => m.category_id === categoryId),
  };
}

/** One category: its format, price and spots, the sign-up while open, and its groups, bracket and schedule once drawn. */
export function CategorySection({ slug, detail, category }: CategorySectionProps) {
  const { tournament, groups } = detail;
  const headingId = `tournament-category-${category.id}`;
  const own = sliceOf(detail, category.id);
  const groupNames = new Map(groups.map((g) => [g.id, g.name]));
  const open = isOpenForSignUp(category) && tournament.status === 'active';
  // A cancelled category keeps only its cancelled state: its draw, if any, is not a result anyone plays for.
  const cancelled = category.status === 'cancelled';
  const hasBracket = !cancelled && own.matches.some((m) => m.stage === 'bracket');
  // Group matches are listed once, in their zone. The schedule is the agenda of the knockout.
  const knockout = own.matches.filter((m) => m.stage === 'bracket');
  const hasSchedule = !cancelled && knockout.some((m) => m.schedule !== null && !m.bye);

  return (
    <section
      aria-labelledby={headingId}
      className="border-border-subtle bg-bg-subtle flex min-w-0 flex-col gap-5 rounded-2xl border p-4 sm:p-5"
    >
      <CategoryHeader headingId={headingId} category={category} sport={tournament.sport} showOffer={open} />

      <SignUpArea
        slug={slug}
        tournament={tournament}
        category={category}
        tournamentActive={tournament.status === 'active'}
      />

      {!cancelled && own.groups.length > 0 && (
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
          <ScheduleList matches={knockout} pairs={own.pairs} groupNames={groupNames} />
        </div>
      )}
    </section>
  );
}
