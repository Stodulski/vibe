import { Ban, Info, Lock, type LucideIcon } from 'lucide-react';
import { Button, formatPrice } from '@vibe/ui';
import { BOOKING_MESSAGES } from '../../messages';
import type { PublicTournament, PublicTournamentDetail, PublicTournamentCategory } from '../../api/types';
import { BracketView } from './BracketView';
import { GroupSection } from './GroupSection';
import { PairSignUpForm } from './PairSignUpForm';
import { ScheduleList } from './ScheduleList';
import { formatLabel, isOpenForSignUp, matchFormatLabel, pairPriceSuffix, spotsLeftLabel } from './tournamentModel';

const t = BOOKING_MESSAGES.tournaments;

function Notice({ icon: Icon, children }: { icon: LucideIcon; children: string }) {
  return (
    <p role="status" className="text-text-secondary text-sm">
      <Icon className="mr-1 inline size-4 align-text-bottom" aria-hidden="true" />
      {children}
    </p>
  );
}

/** Why a category takes no pairs, with a neutral icon: none of these is a success. */
function noticeFor(category: PublicTournamentCategory): { icon: LucideIcon; text: string } {
  if (category.status === 'cancelled') return { icon: Ban, text: t.categoryCancelled };
  if (category.status === 'closed' || category.draw_generated) return { icon: Lock, text: t.signUpClosedCategory };
  return { icon: Info, text: t.categoryFull };
}

interface SignUpAreaProps {
  slug: string;
  tournament: PublicTournament;
  category: PublicTournamentCategory;
  tournamentActive: boolean;
  formOpen: boolean;
  /** Another category's form is open: this category's button steps back to a secondary one. */
  otherFormOpen: boolean;
  onFormOpenChange: (open: boolean) => void;
}

/** The open category offers its form on demand; any other says why it does not. */
function SignUpArea({ slug, tournament, category, tournamentActive, formOpen, otherFormOpen, onFormOpenChange }: SignUpAreaProps) {
  if (!tournamentActive) return null;
  if (!isOpenForSignUp(category)) {
    const notice = noticeFor(category);
    return <Notice icon={notice.icon}>{notice.text}</Notice>;
  }

  return (
    <div className="flex flex-col gap-4">
      {!formOpen && (
        <Button
          size="lg"
          variant={otherFormOpen ? 'outline' : 'default'}
          className="min-h-12 w-full sm:w-auto"
          onClick={() => {
            onFormOpenChange(true);
          }}
        >
          {t.signUp}
        </Button>
      )}
      {formOpen && <PairSignUpForm slug={slug} tournament={tournament} category={category} />}
    </div>
  );
}

interface CategoryHeaderProps {
  headingId: string;
  category: PublicTournamentCategory;
  /** The category takes pairs now: the spots left are worth saying. */
  showOffer: boolean;
}

/** The category's name and format, and, unless it was cancelled, how it is played, its price and its spots. */
function CategoryHeader({ headingId, category, showOffer }: CategoryHeaderProps) {
  const cancelled = category.status === 'cancelled';
  return (
    <div className="flex flex-col gap-1">
      <h2 id={headingId} className="text-text-primary text-lg font-bold">
        {category.name}
      </h2>
      <p className="text-text-secondary text-sm">{formatLabel(category.format)}</p>
      {!cancelled && (
        <>
          <p className="text-text-secondary text-sm">{matchFormatLabel(category)}</p>
          <div className="flex flex-wrap items-baseline justify-between gap-x-4 gap-y-1 pt-1">
            <p className="text-text-primary text-base font-semibold tabular-nums">
              {formatPrice(category.price)}{' '}
              <span className="text-text-secondary text-sm font-normal">{pairPriceSuffix(category)}</span>
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
  /** The category whose sign-up form is open on the page, if any. */
  openFormId: string | null;
  onOpenFormChange: (categoryId: string | null) => void;
}

/** One category: its format, price and spots, the sign-up while open, and its groups, bracket and schedule once drawn. */
export function CategorySection({ slug, detail, category, openFormId, onOpenFormChange }: CategorySectionProps) {
  const { tournament, pairs, groups, matches } = detail;
  const headingId = `tournament-category-${category.id}`;
  const own = {
    pairs: pairs.filter((p) => p.category_id === category.id),
    groups: groups.filter((g) => g.category_id === category.id).sort((a, b) => a.position - b.position),
    matches: matches.filter((m) => m.category_id === category.id),
  };
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
      <CategoryHeader headingId={headingId} category={category} showOffer={open} />

      <SignUpArea
        slug={slug}
        tournament={tournament}
        category={category}
        tournamentActive={tournament.status === 'active'}
        formOpen={openFormId === category.id}
        otherFormOpen={openFormId !== null && openFormId !== category.id}
        onFormOpenChange={(next) => {
          onOpenFormChange(next ? category.id : null);
        }}
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
