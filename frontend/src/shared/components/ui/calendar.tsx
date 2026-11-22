import * as React from 'react';
import { DayPicker, getDefaultClassNames } from 'react-day-picker';
import { es as dayPickerEs } from 'react-day-picker/locale';

import { cn } from '@/shared/lib/utils';
import { Button } from '@/shared/components/ui/button';
import { buildCalendarClassNames } from '@/shared/components/ui/calendar/buildCalendarClassNames';
import {
  CalendarRoot,
  CalendarChevron,
  CalendarWeekNumber,
  CalendarDayButton,
} from '@/shared/components/ui/calendar/CalendarPrimitives';

function Calendar({
  className,
  classNames,
  showOutsideDays = true,
  captionLayout = 'label',
  buttonVariant = 'ghost',
  formatters,
  components,
  locale,
  ...props
}: React.ComponentProps<typeof DayPicker> & {
  buttonVariant?: React.ComponentProps<typeof Button>['variant'];
}) {
  const defaultClassNames = getDefaultClassNames();
  const calendarClassNames = buildCalendarClassNames({
    defaultClassNames,
    captionLayout,
    buttonVariant,
    showWeekNumber: props.showWeekNumber,
  });

  // Callers pass date-fns' `es`, which has no DayPicker labels, so the day
  // buttons and the month arrows were announced in English ("Today, ...").
  // DayPicker's Spanish labels fill whatever the caller's locale leaves out.
  const spanishLocale = { ...dayPickerEs, ...locale, labels: { ...dayPickerEs.labels, ...locale?.labels } };

  return (
    <DayPicker
      locale={spanishLocale}
      showOutsideDays={showOutsideDays}
      className={cn(
        'group/calendar bg-popover p-3 [--cell-size:--spacing(8)] [[data-slot=card-content]_&]:bg-transparent [[data-slot=popover-content]_&]:bg-transparent',
        String.raw`rtl:**:[.rdp-button\_next>svg]:rotate-180`,
        String.raw`rtl:**:[.rdp-button\_previous>svg]:rotate-180`,
        className,
      )}
      captionLayout={captionLayout}
      formatters={{
        formatMonthDropdown: (date) => date.toLocaleString('default', { month: 'short' }),
        ...formatters,
      }}
      classNames={{
        ...calendarClassNames,
        ...classNames,
      }}
      components={{
        Root: CalendarRoot,
        Chevron: CalendarChevron,
        DayButton: CalendarDayButton,
        WeekNumber: CalendarWeekNumber,
        ...components,
      }}
      {...props}
    />
  );
}

export { Calendar, CalendarDayButton };
