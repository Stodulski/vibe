import { useContext, useState } from 'react';
import { SkeletonCancelInfo } from '@vibe/ui';
import { HTTPError } from 'ky';
import { useTournament } from '../hooks/useTournament';
import { useTournamentRegistration } from '../hooks/useTournamentRegistration';
import { useQueryParams } from '../lib/navigation';
import { publicTournamentPath } from '../lib/publicPaths';
import { PublicBookingApiContext } from '../api/context';
import {
  clearTournamentRegistrationToken,
  readTournamentRegistrationToken,
} from '../lib/tournamentRegistrationStorage';
import { CategorySection } from '../components/tournaments/CategorySection';
import { sportLabel, tournamentDates } from '../components/tournaments/tournamentModel';
import { BackToComplexButton, PaymentFailedState, RegistrationNotFoundState } from './event-page/EventPageStates';
import {
  CancelledTournamentNotice,
  TournamentLoadErrorState,
  TournamentNotFoundState,
} from './tournament-page/TournamentPageStates';
import { TournamentRegistrationView } from './tournament-page/TournamentRegistrationView';

const PAGE = 'animate-fade-in mx-auto flex w-full max-w-3xl flex-col gap-6 px-4 py-8 sm:py-12';
const NARROW_PAGE = 'animate-fade-in mx-auto flex w-full max-w-md flex-col gap-6 px-4 py-8 sm:py-12';

/** The registration a returning pair holds: read through the token in the URL. */
function RegistrationPage({ slug, tournamentId, token }: { slug: string; tournamentId: string; token: string }) {
  const [params] = useQueryParams();
  const underReview = params.get('status') === 'pending';
  const { data, isLoading, isError, timedOut, linkNotFound, refetch } = useTournamentRegistration(token);
  const tournamentPath = publicTournamentPath(slug, tournamentId);

  if (linkNotFound) return <RegistrationNotFoundState slug={slug} eventPath={tournamentPath} />;
  if (isLoading) return <SkeletonCancelInfo />;
  if (isError || !data) {
    return (
      <TournamentLoadErrorState
        onRetry={() => {
          void refetch();
        }}
      />
    );
  }
  return (
    <TournamentRegistrationView
      slug={slug}
      token={token}
      tournamentPath={tournamentPath}
      registration={data}
      underReview={underReview}
      timedOut={timedOut}
      onRefresh={() => {
        void refetch();
      }}
    />
  );
}

/** The checkout failed: the held spot is released with the stored token, then the page comes back. */
function usePaymentRetry(tournamentId: string, onDone: () => void) {
  const api = useContext(PublicBookingApiContext);
  const [pending, setPending] = useState(false);
  async function retry() {
    setPending(true);
    const token = readTournamentRegistrationToken(tournamentId);
    try {
      if (token && api) await api.cancelTournamentRegistration({ token });
    } catch {
      // Already released or never held: the page is offered either way.
    }
    clearTournamentRegistrationToken(tournamentId);
    setPending(false);
    onDone();
  }
  return { pending, retry };
}

function TournamentDetailPage({
  slug,
  tournamentId,
  paymentFailed,
}: {
  slug: string;
  tournamentId: string;
  paymentFailed: boolean;
}) {
  const [, setParams] = useQueryParams();
  const [showFailure, setShowFailure] = useState(paymentFailed);
  const [openFormId, setOpenFormId] = useState<string | null>(null);
  const { data: detail, isLoading, error, refetch } = useTournament(slug, tournamentId);
  const { pending, retry } = usePaymentRetry(tournamentId, () => {
    setShowFailure(false);
    setParams(new URLSearchParams());
  });

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
  if (showFailure) return <PaymentFailedState onRetry={() => void retry()} isRetrying={pending} />;

  const { tournament } = detail;
  return (
    <>
      <header className="flex flex-col gap-1">
        <h1 className="text-text-primary text-2xl font-bold">{tournament.name}</h1>
        <p className="text-text-secondary text-sm first-letter:uppercase">{tournamentDates(tournament)}</p>
        <p className="text-text-secondary text-sm">
          {sportLabel(tournament.sport)}
          {detail.complex_name ? `, ${detail.complex_name}` : ''}
        </p>
      </header>
      {tournament.status === 'cancelled' && <CancelledTournamentNotice />}
      {detail.categories.map((category) => (
        <CategorySection
          key={category.id}
          slug={slug}
          detail={detail}
          category={category}
          openFormId={openFormId}
          onOpenFormChange={setOpenFormId}
        />
      ))}
      <BackToComplexButton slug={slug} />
    </>
  );
}

/**
 * `/c/<slug>/tournaments/<id>`: the tournament with its categories, groups,
 * bracket and schedule, the pair sign-up while a category is open and, when
 * the checkout returns here (`?token=...&registered=1`, `&status=pending`, or
 * `?error=payment_failed`), what became of the payment.
 */
export default function TournamentPage({ slug, tournamentId }: { slug: string; tournamentId: string }) {
  const [params] = useQueryParams();
  const token = params.get('token');
  const paymentFailed = params.get('error') === 'payment_failed';

  if (token) {
    return (
      <div className={NARROW_PAGE}>
        <RegistrationPage slug={slug} tournamentId={tournamentId} token={token} />
      </div>
    );
  }
  return (
    <div className={PAGE}>
      <TournamentDetailPage slug={slug} tournamentId={tournamentId} paymentFailed={paymentFailed} />
    </div>
  );
}
