import { useState } from 'react';
import { Button } from '@vibe/ui';
import type { CourtAvailability } from '../../api/types';
import { BOOKING_MESSAGES } from '../../messages';
import { takenSlotOptions } from './takenSlots';
import { WaitlistDialog } from './WaitlistDialog';

const t = BOOKING_MESSAGES.waitlist;

/**
 * After the day's free slots: "¿No está el horario que buscás?". It opens the waitlist
 * popup with a select of the day's taken slots still ahead. Renders nothing when no slot
 * of the day is taken.
 */
export function MissingSlotWaitlist({
  slug,
  date,
  courts,
}: {
  slug: string;
  date: string;
  courts: readonly CourtAvailability[];
}) {
  const [open, setOpen] = useState(false);
  const slots = takenSlotOptions(courts, date);
  if (slots.length === 0) return null;

  return (
    <>
      <Button
        type="button"
        variant="outline"
        size="lg"
        className="mt-4 min-h-12 w-full"
        onClick={() => {
          setOpen(true);
        }}
      >
        {t.missingSlot}
      </Button>
      <WaitlistDialog
        slug={slug}
        request={
          open ? { title: t.slotTitle, description: t.slotDescription, submitLabel: t.sendSlot, slots } : null
        }
        onClose={() => {
          setOpen(false);
        }}
      />
    </>
  );
}
