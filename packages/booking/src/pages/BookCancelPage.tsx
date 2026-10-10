import { SkeletonCancelInfo } from '@vibe/ui';
import { navigateTo, useQueryParams } from '../lib/navigation';
import { publicComplexPath } from '../lib/publicPaths';
import { LinkExpiredState } from '@vibe/ui';
import { InvalidLinkState } from './book-cancel-page/InvalidLinkState';
import { CancelInfoErrorState } from './book-cancel-page/CancelInfoErrorState';
import { CancelledResultState } from './book-cancel-page/CancelledResultState';
import { AlreadyProcessedState } from './book-cancel-page/AlreadyProcessedState';
import { CancelForm } from './book-cancel-page/CancelForm';
import { useCancelBookingFlow } from '../hooks/useCancelBookingFlow';

type CancelBookingFlow = ReturnType<typeof useCancelBookingFlow>;

interface BookCancelPageContentProps {
  token: string;
  flow: CancelBookingFlow;
  goBack: () => void;
}

function BookCancelPageContent({ token, flow, goBack }: BookCancelPageContentProps) {
  const {
    result,
    confirmOpen,
    openConfirm,
    closeConfirm,
    confirmCancel,
    cancelInfo,
    infoLoading,
    cancelInfoError,
    refetchCancelInfo,
    linkExpired,
    linkNotFound,
    isCancelling,
  } = flow;

  if (!token || linkNotFound) {
    return <InvalidLinkState onBack={goBack} />;
  }

  if (linkExpired) {
    return <LinkExpiredState onBack={goBack} />;
  }

  if (result) {
    return <CancelledResultState refund={result} onBack={goBack} />;
  }

  if (infoLoading) {
    return <SkeletonCancelInfo />;
  }

  // Any other resolveLink failure (500, network) is recoverable — unlike a
  // 404/410, retrying can actually succeed — so it gets its own message and
  // a retry instead of the same dead-end "invalid link" messaging.
  if (cancelInfoError) {
    return (
      <CancelInfoErrorState
        onRetry={() => {
          void refetchCancelInfo();
        }}
      />
    );
  }

  if (!cancelInfo) {
    return <InvalidLinkState onBack={goBack} />;
  }

  // Booking already cancelled or completed
  if (!cancelInfo.can_cancel) {
    return <AlreadyProcessedState cancelInfo={cancelInfo} onBack={goBack} />;
  }

  return (
    <CancelForm
      cancelInfo={cancelInfo}
      canRefund={cancelInfo.can_refund}
      confirmOpen={confirmOpen}
      isCancelling={isCancelling}
      onOpenConfirm={openConfirm}
      onCloseConfirm={closeConfirm}
      onConfirmCancel={confirmCancel}
      onBack={goBack}
    />
  );
}

export default function BookCancelPage({ slug }: { slug: string }) {
  const [searchParams] = useQueryParams();
  const token = searchParams.get('token') ?? '';

  const goBack = () => {
    navigateTo(publicComplexPath(slug));
  };

  const flow = useCancelBookingFlow(token);

  return <BookCancelPageContent token={token} flow={flow} goBack={goBack} />;
}
