import { useForm, useFieldArray } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { AlertCircle } from 'lucide-react';
import { LoadingSpinner } from '@/shared/components/common/LoadingSpinner';
import { Button } from '@/shared/components/ui/button';
import { updateSchedulesSchema, type UpdateSchedulesDto } from '../schemas/complex.schema';
import { useSchedules } from '../hooks/useSchedules';
import { useUpdateSchedules } from '../hooks/useUpdateSchedules';
import { SectionFooter } from '@/shared/components/common/SectionFooter';
import { ES_AR } from '@/shared/i18n/es_AR';
import { submitHandler } from '@/shared/lib/form';
import { DAYS } from './schedule-config/days';
import { ScheduleDayRow } from './schedule-config/ScheduleDayRow';
import type { Schedule } from '@/shared/types/api.types';

const t = ES_AR;

interface ScheduleConfigProps {
  complexId: string;
  slug: string | undefined;
}

function orderedSchedules(schedules: Schedule[]): UpdateSchedulesDto['schedules'] {
  return DAYS.map((day) => {
    const s = schedules.find((sch) => sch.day === day);
    return {
      day,
      open_time: s?.open_time ?? '08:00',
      close_time: s?.close_time ?? '23:00',
      is_closed: s?.is_closed ?? false,
    };
  });
}

/**
 * Loads the real schedules before anything resembling a form appears.
 *
 * `useSchedules` used to feed a form that was *always* mounted with
 * 08:00-23:00 defaults and reset once (if ever) the query resolved. A
 * disabled query (no `slug` yet) or a failed one both leave `data`
 * `undefined` while `isLoading` reads `false` — so the defaults sat there,
 * editable, with "Guardar" enabled, ready to overwrite real opening hours
 * with a fabricated week. There is now no `<form>` at all until real data
 * exists: no slug and no loading state both render explicit non-form states,
 * a failed fetch renders a retry, and only `data` mounts `ScheduleForm` (with
 * `defaultValues` seeded once from the loaded schedules, keyed by
 * `complexId` so switching complexes remounts it instead of reusing stale
 * field values).
 */
export function ScheduleConfig({ complexId, slug }: ScheduleConfigProps) {
  const { data: schedules, isLoading, isError, refetch } = useSchedules(complexId, slug);

  if (!slug || isLoading) {
    return <LoadingSpinner size="md" className="py-12" />;
  }

  if (isError || !schedules) {
    return (
      <div className="flex flex-col items-center justify-center gap-3 rounded-lg border border-border-subtle bg-bg-subtle p-12">
        <AlertCircle className="size-8 text-text-tertiary" />
        <p className="text-sm text-text-tertiary">{t.common.error}</p>
        <Button
          variant="outline"
          size="sm"
          onClick={() => {
            void refetch();
          }}
        >
          {t.common.refresh}
        </Button>
      </div>
    );
  }

  return <ScheduleForm key={complexId} complexId={complexId} schedules={schedules} />;
}

function ScheduleForm({ complexId, schedules }: { complexId: string; schedules: Schedule[] }) {
  const updateSchedules = useUpdateSchedules(complexId);

  const { control, handleSubmit, setValue } = useForm<UpdateSchedulesDto>({
    resolver: zodResolver(updateSchedulesSchema),
    defaultValues: { schedules: orderedSchedules(schedules) },
  });

  const { fields } = useFieldArray({ control, name: 'schedules' });

  const onSubmit = (data: UpdateSchedulesDto) => {
    updateSchedules.mutate(data);
  };

  return (
    <form onSubmit={submitHandler(handleSubmit, onSubmit)} className="space-y-5">
      {/* A list, not a table. Seven rows of a day, two times and a checkbox do
          not need column headings: the day names itself, a time control looks
          like a time control, and the checkbox says "Cerrado" beside it. The
          headings were a caption for a picture that already spoke.

          No container per row either — proximity, similarity and a hairline
          between rows group them without a box around every line. */}
      <div>
        {fields.map((field, index) => (
          <ScheduleDayRow key={field.id} index={index} control={control} setValue={setValue} />
        ))}
      </div>

      <SectionFooter submitLabel={t.common.save} pending={updateSchedules.isPending} align="start" />
    </form>
  );
}
