import { useState } from 'react';
import { Repeat } from 'lucide-react';
import { Sheet, SheetContent, SheetFooter } from '@/shared/components/ui/sheet';
import { Button } from '@/shared/components/ui/button';
import { DetailSheetHeader } from '@/shared/components/common/DetailSheetHeader';
import { InfoRow } from '@/shared/components/common/InfoRow';
import { useExitingValue } from '@/shared/hooks/useExitingValue';
import { useOutsideClickGrace } from '@/shared/hooks/useOutsideClickGrace';
import { ES_AR } from '@/shared/i18n/es_AR';
import { formatEventDay, formatEventTimeRange } from '@/shared/lib/eventDates';
import { SkipDateDialog } from './booking-detail/SkipDateDialog';
import { EndSeriesDialog } from './booking-detail/EndSeriesDialog';
import { weekdayOf } from '../lib/seriesDates';
import { todayInArgentina } from '../lib/today';
import type { FixedBlock } from '@/shared/types/api.types';

const t = ES_AR;

interface FixedBlockDetailProps {
  open: boolean;
  onClose: () => void;
  /** The grid block the owner pressed; null while closed. */
  block: FixedBlock | null;
  courtName: string;
  complexId: string;
}

function FixedBlockFacts({ block, date, courtName }: { block: FixedBlock; date: string; courtName: string }) {
  const weekday = t.bookings.series.weekdaysPlural[weekdayOf(date)] ?? '';
  return (
    <div className="min-h-0 flex-1 space-y-4 overflow-y-auto px-4 pt-4 pb-8 sm:px-6">
      <div className="flex h-5 items-center gap-2">
        <Repeat className="text-text-tertiary size-4 shrink-0" aria-hidden="true" />
        <p className="text-text-secondary min-w-0 text-sm break-words">
          {t.bookings.series.label(weekday, block.start_time.slice(0, 5))}
        </p>
      </div>
      <InfoRow label={t.bookings.client}>
        <p className="break-words">{block.client_name}</p>
      </InfoRow>
      <InfoRow label={t.bookings.court}>{courtName}</InfoRow>
      <InfoRow label={t.bookings.time}>
        <span className="score-text font-medium">{formatEventTimeRange(block.start_time, block.end_time)}</span>
      </InfoRow>
    </div>
  );
}

function FixedBlockActions({ onSkip, onEnd }: { onSkip: () => void; onEnd: () => void }) {
  return (
    <SheetFooter className="items-start sm:px-6">
      <Button
        variant="outline"
        className="border-error-text/30 text-error-text hover:bg-error-text/10 rounded-lg"
        onClick={onSkip}
      >
        {t.bookings.series.cancelOne}
      </Button>
      <Button variant="destructive" className="rounded-lg" onClick={onEnd}>
        {t.bookings.series.endFromHere}
      </Button>
    </SheetFooter>
  );
}

/** The two confirmations behind the sheet: free just this date, or end the series from it. */
function FixedBlockDialogs({
  block,
  complexId,
  skipOpen,
  endOpen,
  onCloseSkip,
  onCloseEnd,
  onDone,
}: {
  block: FixedBlock;
  complexId: string;
  skipOpen: boolean;
  endOpen: boolean;
  onCloseSkip: () => void;
  onCloseEnd: () => void;
  /** Shuts the sheet once either one went through. */
  onDone: () => void;
}) {
  const date = block.date.slice(0, 10);
  return (
    <>
      <SkipDateDialog
        seriesId={block.series_id}
        date={date}
        complexId={complexId}
        open={skipOpen}
        onClose={onCloseSkip}
        onSkipped={onDone}
      />
      <EndSeriesDialog
        seriesId={block.series_id}
        fromDate={date}
        complexId={complexId}
        open={endOpen}
        onClose={onCloseEnd}
        onEnded={onDone}
      />
    </>
  );
}

/**
 * What a fixed weekly booking's date offers while it has no booking yet:
 * free just this date, or cancel the whole turno fijo from it on. Built from
 * the same pieces as the blocked-slot drawer.
 */
export function FixedBlockDetail({ open, onClose, block, courtName, complexId }: FixedBlockDetailProps) {
  const [endOpen, setEndOpen] = useState(false);
  const [skipOpen, setSkipOpen] = useState(false);
  const onPointerDownOutside = useOutsideClickGrace(open);
  // Keeps the last block on screen while the sheet slides away (see `BlockedSlotDetail`).
  const shown = useExitingValue(block);
  if (!shown) return null;

  const date = shown.date.slice(0, 10);
  // The server refuses a date in the past, so there is nothing to offer for one.
  const canCancel = date >= todayInArgentina();

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
          <DetailSheetHeader
            title={t.bookings.series.blockDetailTitle}
            subtitle={<span className="inline-block first-letter:uppercase">{formatEventDay(date)}</span>}
          />

          <FixedBlockFacts block={shown} date={date} courtName={courtName} />

          {canCancel && (
            <FixedBlockActions
              onSkip={() => {
                setSkipOpen(true);
              }}
              onEnd={() => {
                setEndOpen(true);
              }}
            />
          )}
        </SheetContent>
      </Sheet>
      <FixedBlockDialogs
        block={shown}
        complexId={complexId}
        skipOpen={skipOpen}
        endOpen={endOpen}
        onCloseSkip={() => {
          setSkipOpen(false);
        }}
        onCloseEnd={() => {
          setEndOpen(false);
        }}
        onDone={onClose}
      />
    </>
  );
}
