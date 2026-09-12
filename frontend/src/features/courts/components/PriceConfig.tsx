import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/shared/components/common/AppDialog';
import { SectionFooter } from '@/shared/components/common/SectionFooter';
import { ES_AR } from '@/shared/i18n/es_AR';
import { submitHandler } from '@/shared/lib/form';
import { DayRow } from './price-config/DayRow';
import { usePriceConfigForm } from './price-config/usePriceConfigForm';
import { ALL_DAYS } from './price-config/days';
import type { CourtWithPrices } from '@/shared/types/api.types';

const t = ES_AR;

interface PriceConfigProps {
  open: boolean;
  onClose: () => void;
  complexId: string;
  court: CourtWithPrices;
}

export function PriceConfig({ open, onClose, complexId, court }: PriceConfigProps) {
  const { form, onSubmit, isPending } = usePriceConfigForm(complexId, court, onClose);
  const {
    register,
    handleSubmit,
    setValue,
    getValues,
    control,
    formState: { errors },
  } = form;

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

        <form onSubmit={submitHandler(handleSubmit, onSubmit)} className="space-y-5">
          <p className="text-xs text-text-tertiary">{t.courts.pricesHourlyHint}</p>

          {/* One table, not two boxes. Split weekday/weekend it was five rows
              beside two, which left a hole the size of three rows in the
              bottom right — the days are named, so the grouping was costing
              more than it said. Seven rows flow down one column and into the
              next, four and three, which balances and uses the width. */}
          <section className="rounded-xl border border-border-subtle bg-bg-elevated p-2">
            <div className="grid grid-cols-1 gap-x-4 sm:grid-flow-col sm:grid-cols-2 sm:grid-rows-4">
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
                  errors={errors}
                />
              ))}
            </div>
          </section>

          <SectionFooter onCancel={onClose} submitLabel={t.common.save} pending={isPending} />
        </form>
      </DialogContent>
    </Dialog>
  );
}
