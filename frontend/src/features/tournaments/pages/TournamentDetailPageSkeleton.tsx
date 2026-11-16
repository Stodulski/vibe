import { BackLinkSkeleton } from '@/shared/components/common/BackLinkSkeleton';
import { PageHeaderSkeleton } from '@/shared/components/common/PageHeaderSkeleton';
import { TournamentDetailBodySkeleton } from '../components/TournamentDetailBodySkeleton';
import { TOURNAMENT_DETAIL_CLASS } from '../lib/tournamentLayout';

/** What `TournamentDetailPage` draws while it loads: the back link, the header, then the body. */
export function TournamentDetailPageSkeleton() {
  return (
    <div className={TOURNAMENT_DETAIL_CLASS}>
      <BackLinkSkeleton />
      <PageHeaderSkeleton />
      <TournamentDetailBodySkeleton />
    </div>
  );
}
