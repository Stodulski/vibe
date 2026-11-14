import { CheckCircle2, LogOut } from 'lucide-react';
import { Button, StatusHero, formatDateFull } from '@vibe/ui';
import { BOOKING_MESSAGES } from '../../messages';
import { useCancelTournamentRegistration } from '../../hooks/useTournamentRegistration';
import { CancelRegistration, CancelledResult, type CancelCopy } from '../../components/registration/CancelRegistration';
import { formatLabel } from '../../components/tournaments/tournamentModel';
import { BackToComplexButton, ExpiredState, ProcessingState } from '../event-page/EventPageStates';
import type { PublicPairRegistration } from '../../api/types';

const t = BOOKING_MESSAGES;

const CANCEL_COPY: CancelCopy = {
  action: t.tournaments.cancelPair,
  confirmTitle: t.tournaments.cancelConfirmTitle,
  confirmRefund: t.events.cancelConfirmRefund,
  confirmNoRefund: t.events.cancelConfirmNoRefund,
  confirmWalkover: t.tournaments.cancelConfirmWalkover,
  walkoverNote: t.tournaments.walkoverNote,
};

function PairSummary({ registration }: { registration: PublicPairRegistration }) {
  const { tournament, category, payer, partner } = registration;
  return (
    <div className="border-border-subtle bg-bg-subtle flex w-full flex-col gap-2 rounded-2xl border p-4 text-left">
      <h2 className="text-text-primary text-lg font-bold">{tournament.name}</h2>
      <p className="text-text-secondary text-sm first-letter:uppercase">{formatDateFull(tournament.start_date)}</p>
      <p className="text-text-secondary text-sm">
        <span className="text-text-tertiary">{t.tournaments.categoryName}: </span>
        {category.name} <span className="text-text-tertiary">{formatLabel(category.format)}</span>
      </p>
      <div className="text-text-primary text-sm">
        <span className="text-text-tertiary">{t.tournaments.pairLabel}</span>
        <p>{`${payer.first_name} ${payer.last_name}`}</p>
        <p>{`${partner.first_name} ${partner.last_name}`}</p>
      </div>
    </div>
  );
}

function ConfirmedView({
  registration,
  tournamentPath,
  cancel,
}: {
  registration: PublicPairRegistration;
  tournamentPath: string;
  cancel: ReturnType<typeof useCancelTournamentRegistration>;
}) {
  if (registration.pair_status === 'withdrawn') {
    return (
      <StatusHero
        icon={LogOut}
        tone="neutral"
        title={t.tournaments.withdrawnTitle}
        description={t.tournaments.withdrawnDescription}
      >
        <Button asChild variant="outline" className="min-h-12">
          <a href={tournamentPath}>{t.tournaments.backToTournament}</a>
        </Button>
      </StatusHero>
    );
  }
  const { cancellation } = registration;
  return (
    <StatusHero
      icon={CheckCircle2}
      tone="success"
      size="large"
      title={t.tournaments.confirmedTitle}
      description={t.tournaments.confirmedDescription}
      descriptionClassName="max-w-sm"
    >
      <div className="flex w-full max-w-sm flex-col gap-6">
        <PairSummary registration={registration} />
        <Button asChild variant="outline" className="min-h-12">
          <a href={tournamentPath}>{t.tournaments.backToTournament}</a>
        </Button>
        {cancellation.can_cancel && (
          <CancelRegistration
            withinWindow={cancellation.within_window}
            deadline={cancellation.deadline}
            walkover={cancellation.walkover}
            cancel={cancel}
            copy={CANCEL_COPY}
          />
        )}
      </div>
    </StatusHero>
  );
}

interface TournamentRegistrationViewProps {
  slug: string;
  token: string;
  tournamentPath: string;
  registration: PublicPairRegistration;
  /** `?status=pending`: the checkout returned while the payment is still under review. */
  underReview: boolean;
  timedOut: boolean;
  onRefresh: () => void;
}

/** The pair's own spot after the checkout returns: one state per registration status. */
export function TournamentRegistrationView({
  slug,
  token,
  tournamentPath,
  registration,
  underReview,
  timedOut,
  onRefresh,
}: TournamentRegistrationViewProps) {
  const cancelled = useCancelTournamentRegistration(token);
  if (cancelled.data) {
    return <CancelledResult refund={cancelled.data.refund} slug={slug} title={t.tournaments.cancelledResultTitle} />;
  }

  switch (registration.status) {
    case 'confirmed':
      return <ConfirmedView registration={registration} tournamentPath={tournamentPath} cancel={cancelled} />;
    case 'cancelled':
      return (
        <StatusHero
          icon={CheckCircle2}
          tone="neutral"
          title={t.tournaments.cancelledByYouTitle}
          description={t.publicBooking.cancelledSuccessfully}
        >
          <BackToComplexButton slug={slug} />
        </StatusHero>
      );
    case 'expired':
      return <ExpiredState eventPath={tournamentPath} slug={slug} />;
    default:
      return <ProcessingState underReview={underReview} timedOut={timedOut} onRefresh={onRefresh} />;
  }
}
