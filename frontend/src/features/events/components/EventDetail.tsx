import { useState } from 'react';
import { Sheet, SheetContent, SheetFooter } from '@/shared/components/ui/sheet';
import { Button } from '@/shared/components/ui/button';
import { DetailSheetHeader } from '@/shared/components/common/DetailSheetHeader';
import { InfoRow } from '@/shared/components/common/InfoRow';
import { useExitingValue } from '@/shared/hooks/useExitingValue';
import { useOutsideClickGrace } from '@/shared/hooks/useOutsideClickGrace';
import { ES_AR } from '@/shared/i18n/es_AR';
import { formatDateFull, formatDateShort, formatPrice, formatTime, getHttpErrorMessage } from '@/shared/lib/utils';
import { todayInArgentina } from '@/shared/lib/today';
import { useEvent } from '../hooks/useEvent';
import { EVENT_DETAIL_BODY_CLASS } from '../lib/eventLayout';
import { paidCount } from '../lib/participantStatus';
import { CancelEventDialog } from './CancelEventDialog';
import { EventDetailSkeleton } from './EventDetailSkeleton';
import { ParticipantList } from './ParticipantList';
import type { OpenPlayEvent, OpenPlayParticipant } from '@/shared/types/api.types';

const t = ES_AR.events;

interface EventDetailProps {
  open: boolean;
  onClose: () => void;
  complexId: string;
  /** The event pressed on the list or the grid; null while closed. */
  eventId: string | null;
}

function EventFacts({ event }: { event: OpenPlayEvent }) {
  return (
    <>
      {event.status === 'cancelled' && (
        <p className="text-error-text text-sm">
          {t.detail.cancelledNote}
          {event.cancelled_at && ` ${t.detail.cancelledAt(formatDateShort(event.cancelled_at))}.`}
        </p>
      )}
      <InfoRow label={t.detail.when}>
        <span className="first-letter:uppercase">{formatDateFull(event.date)}</span>
        {' · '}
        <span className="score-text font-medium">
          {formatTime(event.start_time)} – {formatTime(event.end_time)}
        </span>
      </InfoRow>
      <InfoRow label={t.detail.courts}>{event.courts.map((court) => court.name).join(', ')}</InfoRow>
      <InfoRow label={t.detail.capacity}>{t.spotsTaken(event.confirmed_count, event.capacity)}</InfoRow>
      <InfoRow label={t.detail.price}>
        <span className="score-text font-medium">{formatPrice(event.price)}</span>
      </InfoRow>
    </>
  );
}

function EventBody({ event, participants }: { event: OpenPlayEvent; participants: OpenPlayParticipant[] }) {
  return (
    <div className={EVENT_DETAIL_BODY_CLASS}>
      <EventFacts event={event} />
      <section className="pt-2">
        <h3 className="text-text-primary mb-1 text-sm font-semibold">
          {t.detail.participants} · {t.detail.participantsCount(participants.length)}
        </h3>
        <ParticipantList participants={participants} />
      </section>
    </div>
  );
}

/**
 * The owner's view of one event: its facts, everyone who held a spot with their
 * payment, and the cancel action while the event is still ahead.
 */
export function EventDetail({ open, onClose, complexId, eventId }: EventDetailProps) {
  const [cancelOpen, setCancelOpen] = useState(false);
  const onPointerDownOutside = useOutsideClickGrace(open);
  // Keeps the last event on screen while the sheet slides away.
  const shownId = useExitingValue(eventId);
  const { data, isLoading, error } = useEvent(complexId, shownId);
  const event = data?.event;
  const participants = data?.participants ?? [];
  // Nothing left to cancel once the day is over or it already was.
  const canCancel = !!event && event.status === 'active' && event.date >= todayInArgentina();

  return (
    <>
      <Sheet
        open={open}
        onOpenChange={(v) => {
          if (!v) onClose();
        }}
      >
        <SheetContent
          side="right"
          className="sm:max-w-lg"
          showCloseButton={false}
          onPointerDownOutside={onPointerDownOutside}
        >
          <DetailSheetHeader title={event?.name ?? t.title} />
          {isLoading && <EventDetailSkeleton />}
          {!!error && (
            <p role="alert" className="text-error-text px-4 pt-4 text-sm sm:px-6">
              {getHttpErrorMessage(error, t.detail.loadError)}
            </p>
          )}
          {event && <EventBody event={event} participants={participants} />}
          {canCancel && (
            <SheetFooter>
              <Button
                variant="destructive"
                className="w-full rounded-lg"
                onClick={() => {
                  setCancelOpen(true);
                }}
              >
                {t.cancel.action}
              </Button>
            </SheetFooter>
          )}
        </SheetContent>
      </Sheet>
      {shownId && (
        <CancelEventDialog
          open={cancelOpen}
          onClose={() => {
            setCancelOpen(false);
          }}
          complexId={complexId}
          eventId={shownId}
          paidCount={paidCount(participants)}
        />
      )}
    </>
  );
}
