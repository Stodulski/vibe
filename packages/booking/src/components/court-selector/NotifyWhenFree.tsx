import { useState } from 'react';
import { Button } from '@vibe/ui';
import { BOOKING_MESSAGES } from '../../messages';
import { WaitlistDialog, type WaitlistRequest } from '../waitlist/WaitlistDialog';
import type { RulerHour } from './rulerHours';
import { waitlistOf } from './waitlistOf';

const w = BOOKING_MESSAGES.waitlist;

interface NotifyWhenFreeProps {
  hour: RulerHour;
  /** The complex and day the wait is for. */
  slug: string;
  date: string;
}

/** "Avisarme cuando se libere" under a taken hour, and the popup it opens. */
export function NotifyWhenFree({ hour, slug, date }: NotifyWhenFreeProps) {
  const [request, setRequest] = useState<WaitlistRequest | null>(null);
  return (
    <>
      <Button
        variant="outline"
        size="lg"
        className="min-h-12 w-full"
        onClick={() => {
          setRequest(waitlistOf(hour, date));
        }}
      >
        {w.sendSlot}
      </Button>
      <WaitlistDialog
        slug={slug}
        request={request}
        onClose={() => {
          setRequest(null);
        }}
      />
    </>
  );
}
