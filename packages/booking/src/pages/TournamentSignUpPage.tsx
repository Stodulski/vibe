import type { ReactNode } from 'react';
import { SkeletonCancelInfo } from '@vibe/ui';
import { HTTPError } from 'ky';
import { useTournament } from '../hooks/useTournament';
import { useQueryParams } from '../lib/navigation';
import { publicTournamentPath } from '../lib/publicPaths';
import { BOOKING_MESSAGES } from '../messages';
import { CheckoutBackLink, CheckoutLayout, type CheckoutBack } from '../components/CheckoutLayout';
import { PairSignUpForm } from '../components/tournaments/PairSignUpForm';
import { PairSummary } from '../components/tournaments/PairSummary';
import { Notice } from '../components/tournaments/CategorySection';
import { noticeFor } from '../components/tournaments/signUpNotice';
import { isOpenForSignUp } from '../components/tournaments/tournamentModel';
import {
  CancelledTournamentNotice,
  TournamentLoadErrorState,
  TournamentNotFoundState,
} from './tournament-page/TournamentPageStates';

const t = BOOKING_MESSAGES;

/** The category was chosen on the tournament page, so the checkout's progress starts at the data. */
const TOURNAMENT_STEPS = [t.tournaments.stepCategory, t.publicBooking.stepData, t.publicBooking.stepPay] as const;

/** A state in which the category takes no pair: the way back, and what it is in place of the form. */
function Unavailable({ back, children }: { back: CheckoutBack; children?: ReactNode }) {
  return (
    <div className="animate-fade-in w-full space-y-6 sm:space-y-8">
      <CheckoutBackLink {...back} />
      {children}
    </div>
  );
}

/**
 * `/c/<slug>/tournaments/<id>/inscripcion?category=<id>`: the pair sign-up for
 * one category, in the checkout's own column, with the way back to the
 * tournament. A category that takes no pair says why instead of the form.
 */
export default function TournamentSignUpPage({ slug, tournamentId }: { slug: string; tournamentId: string }) {
  const [params] = useQueryParams();
  const categoryId = params.get('category');
  const { data: detail, isLoading, error, refetch } = useTournament(slug, tournamentId);
  const back = { label: t.tournaments.backToTournamentLink, href: publicTournamentPath(slug, tournamentId) };

  if (isLoading) return <SkeletonCancelInfo />;
  if (error instanceof HTTPError && error.response.status === 404) return <TournamentNotFoundState slug={slug} />;
  if (error || !detail) {
    return (
      <TournamentLoadErrorState
        onRetry={() => {
          void refetch();
        }}
      />
    );
  }

  if (detail.tournament.status === 'cancelled') {
    return (
      <Unavailable back={back}>
        <CancelledTournamentNotice />
      </Unavailable>
    );
  }
  const category = detail.categories.find((c) => c.id === categoryId);
  if (!category) return <Unavailable back={back} />;
  if (!isOpenForSignUp(category)) {
    const notice = noticeFor(category);
    return (
      <Unavailable back={back}>
        <Notice icon={notice.icon}>{notice.text}</Notice>
      </Unavailable>
    );
  }

  return (
    <CheckoutLayout
      currentStep={2}
      steps={TOURNAMENT_STEPS}
      back={back}
      summary={
        <PairSummary
          tournament={detail.tournament}
          category={category}
          venue={detail.complex_name}
          cancellationHours={detail.cancellation_hours}
        />
      }
    >
      <PairSignUpForm slug={slug} tournament={detail.tournament} category={category} />
    </CheckoutLayout>
  );
}
