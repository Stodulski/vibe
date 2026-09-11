import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription } from '@/shared/components/ui/dialog';
import { SectionFooter } from '@/shared/components/common/SectionFooter';
import { ES_AR } from '@/shared/i18n/es_AR';
import { DateField, CourtField, TimeRangeFields, ReasonField } from './block-slot-modal/fields';
import { useBlockSlotForm, type BlockSlotPrefill } from './block-slot-modal/useBlockSlotForm';
import type { CourtWithPrices } from '@/shared/types/api.types';

const t = ES_AR;

interface BlockSlotModalProps {
  open: boolean;
  onClose: () => void;
  complexId: string;
  courts: CourtWithPrices[];
  prefill?: BlockSlotPrefill;
}

/**
 * Mounted only while `open`, keyed by `prefill`.
 *
 * The old six-`useState` version copied `prefill` into its initial state once
 * and never again — a parent changing `prefill` while the modal stayed
 * mounted was silently ignored until the next close/reopen cycle. Remounting
 * on a new `prefill` (or a fresh open) gets a form built from the current
 * prefill every time, the same "key instead of a reset effect" pattern as
 * `ScheduleConfig`/`CourtForm`.
 */
export function BlockSlotModal({ open, onClose, complexId, courts, prefill }: BlockSlotModalProps) {
  return (
    <Dialog
      open={open}
      onOpenChange={(v) => {
        if (!v) onClose();
      }}
    >
      <DialogContent className="sm:max-w-md">
        {open && (
          <BlockSlotFormBody
            key={JSON.stringify(prefill ?? null)}
            complexId={complexId}
            courts={courts}
            prefill={prefill}
            onClose={onClose}
          />
        )}
      </DialogContent>
    </Dialog>
  );
}

function BlockSlotFormBody({
  complexId,
  courts,
  prefill,
  onClose,
}: {
  complexId: string;
  courts: CourtWithPrices[];
  prefill: BlockSlotPrefill | undefined;
  onClose: () => void;
}) {
  const form = useBlockSlotForm(complexId, courts, prefill, onClose);

  return (
    <>
      <DialogHeader>
        <DialogTitle>{t.courts.block}</DialogTitle>
        <DialogDescription>{t.courts.blockDescription}</DialogDescription>
      </DialogHeader>

      <div className="space-y-5">
        <DateField
          date={form.date}
          error={form.errors.date?.message}
          calendarOpen={form.calendarOpen}
          setCalendarOpen={form.setCalendarOpen}
          onSelectDate={form.handleSelectDate}
        />
        <CourtField
          courtId={form.courtId}
          error={form.errors.court_id?.message}
          setCourtId={form.setCourtId}
          activeCourts={form.activeCourts}
        />
        <TimeRangeFields
          date={form.date}
          startTime={form.startTime}
          endTime={form.endTime}
          startTimeError={form.errors.start_time?.message}
          endTimeError={form.errors.end_time?.message}
          startTimeOptions={form.startTimeOptions}
          endTimeOptions={form.endTimeOptions}
          setStartTime={form.setStartTime}
          setEndTime={form.setEndTime}
        />
        <ReasonField reason={form.reason} setReason={form.setReason} />
      </div>

      {/* Actions */}
      <SectionFooter
        onCancel={form.handleClose}
        cancelDisabled={form.isPending}
        onSubmit={() => {
          void form.handleSubmit();
        }}
        submitDisabled={!form.isValid}
        pending={form.isPending}
        submitLabel={t.courts.block}
      />
    </>
  );
}
