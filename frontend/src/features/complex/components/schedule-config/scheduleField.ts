import type { UpdateSchedulesDto } from '../../schemas/complex.schemas';
import type { UseFormReturn } from 'react-hook-form';

export type ScheduleFormControl = UseFormReturn<UpdateSchedulesDto>['control'];
export type ScheduleFormSetValue = UseFormReturn<UpdateSchedulesDto>['setValue'];

/**
 * Builds a typed react-hook-form field path for a schedule row.
 * `String(idx)` keeps `restrict-template-expressions` happy while the `as`
 * cast restores the literal `schedules.${number}.${K}` type RHF expects.
 */
export function scheduleField<K extends 'day' | 'open_time' | 'close_time' | 'is_closed'>(
  idx: number,
  key: K,
): `schedules.${number}.${K}` {
  return `schedules.${String(idx)}.${key}` as `schedules.${number}.${K}`;
}
