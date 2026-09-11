import * as React from 'react';
import { DayPicker, getDefaultClassNames } from 'react-day-picker';

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

  return (
    <DayPicker
      showOutsideDays={showOutsideDays}
      className={cn(
        'group/calendar bg-background p-3 [--cell-size:--spacing(8)] [[data-slot=card-content]_&]:bg-transparent [[data-slot=popover-content]_&]:bg-transparent',
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
