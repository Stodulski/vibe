import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/shared/components/common/AppDialog';
import { LoadingSpinner } from '@/shared/components/common/LoadingSpinner';
import { SectionFooter } from '@/shared/components/common/SectionFooter';
import { useComplex, useSchedules } from '@/features/complex';
import { ES_AR } from '@/shared/i18n/es_AR';
import { submitHandler } from '@/shared/lib/form';
import { DayRow } from './price-config/DayRow';
import { usePriceConfigForm } from './price-config/usePriceConfigForm';
import { ALL_DAYS } from './price-config/days';
import type { CourtWithPrices, Schedule } from '@/shared/types/api.types';

const t = ES_AR;

interface PriceConfigProps {
  open: boolean;
  onClose: () => void;
  complexId: string;
  court: CourtWithPrices;
}

/**
 * The rate table, once the venue's opening hours are known.
 *
 * The hours are not decoration here: they seed the first band of every day, and
 * they are what the owner reads and edits. A form mounted before the schedules
 * query resolves would show 00:00–23:59 for a venue that opens at eight and
 * save it if the owner pressed Guardar — the same fabricated week
 * `ScheduleConfig` refuses to mount its own form over. Both queries are cached
 * by the time this dialog is normally reached; the wait is for the case where
 * they are not.
 *
 * Only a query still IN FLIGHT holds the form back. A complex with no slug
 * leaves `useSchedules` disabled forever and a failed fetch never resolves, so
 * gating on "has data" would leave a spinner the owner can do nothing with.
 * Both of those fall through to the all-day default instead, which is what this
 * dialog wrote for every day before it read schedules at all.
 */
export function PriceConfig({ open, onClose, complexId, court }: PriceConfigProps) {
  const { data: complex, isLoading: complexLoading } = useComplex(complexId);
  const { data: schedules, isLoading: schedulesLoading } = useSchedules(complexId, complex?.slug);
  const isLoading = complexLoading || schedulesLoading;

  return (
    <Dialog
      open={open}
      onOpenChange={(v) => {
        if (!v) onClose();
      }}
    >
      <DialogContent className="sm:max-h-[90dvh] sm:max-w-2xl">
        <DialogHeader>
          <DialogTitle>
            {t.courts.prices} · {court.name}
          </DialogTitle>
        </DialogHeader>

        {isLoading ? (
          <LoadingSpinner className="py-12" />
        ) : (
          <PriceForm complexId={complexId} court={court} schedules={schedules ?? []} onClose={onClose} />
        )}
      </DialogContent>
    </Dialog>
  );
}

function PriceForm({
  complexId,
  court,
  schedules,
  onClose,
}: {
  complexId: string;
  court: CourtWithPrices;
  schedules: Schedule[];
  onClose: () => void;
}) {
  const { form, onSubmit, nextBand, isPending } = usePriceConfigForm(complexId, court, schedules, onClose);
  const {
    register,
    handleSubmit,
    setValue,
    getValues,
    control,
    formState: { errors },
  } = form;

  return (
    <form onSubmit={submitHandler(handleSubmit, onSubmit)} className="space-y-5">
      <p className="text-text-tertiary text-xs">{t.courts.pricesHourlyHint}</p>

      {/* One table, not two boxes. Split weekday/weekend it was five rows
          beside two, which left a hole the size of three rows in the
          bottom right — the days are named, so the grouping was costing
          more than it said.

          One column, not the two it used to flow into: a day can now grow
          into several band rows, and two columns of independently-growing
          lists put Thursday's second band beside Sunday's first with no
          reading order between them. Down one column the days stay in the
          order of the week whatever any of them expands to.

          No card around the list either, on top of that: no border, fill or
          rounding of its own, and no padding beyond what `DialogContent`
          already gives its content — the rows sit on the dialog's own
          background now, flush with the title and footer above and below
          them, instead of inside a second box drawn over the first. The
          hairline between one day and the next is still there; it just lives
          on each `DayRow` (`border-b`), not on a wrapper around all of them. */}
      <section>
        {ALL_DAYS.map(({ value, label, short }) => (
          <DayRow
            key={value}
            day={value}
            label={label}
            shortLabel={short}
            control={control}
            register={register}
            setValue={setValue}
            getValues={getValues}
            nextBand={nextBand}
            errors={errors}
          />
        ))}
      </section>

      <SectionFooter onCancel={onClose} submitLabel={t.common.save} pending={isPending} />
    </form>
  );
}
