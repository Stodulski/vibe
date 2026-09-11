import { useState } from 'react';
import type { UseFormRegister } from 'react-hook-form';
import { ChevronDown } from 'lucide-react';
import { Textarea } from '@/shared/components/ui/textarea';
import { Label } from '@/shared/components/ui/label';
import { ES_AR } from '@/shared/i18n/es_AR';
import { cn } from '@/shared/lib/utils';
import type { PublicBookingFormData } from '../../schemas/public-booking.schemas';

const NOTES_TEXTAREA_ID = 'client_notes';

const t = ES_AR;

interface NotesFieldProps {
  register: UseFormRegister<PublicBookingFormData>;
}

export function NotesField({ register }: NotesFieldProps) {
  const [notesOpen, setNotesOpen] = useState(false);

  return (
    <div>
      <button
        type="button"
        onClick={() => {
          setNotesOpen((v) => !v);
        }}
        className="flex min-h-[44px] items-center gap-1.5 text-sm text-text-tertiary hover:text-text-secondary transition-colors"
        aria-expanded={notesOpen}
        aria-controls={NOTES_TEXTAREA_ID}
      >
        <ChevronDown className={cn('size-4 transition-transform', notesOpen && 'rotate-180')} />
        {t.publicBooking.notes}
        <span>({t.publicBooking.optional})</span>
      </button>
      {notesOpen && (
        <>
          {/* The toggle button above already shows "Notas" visibly; this label
              only needs to associate the textarea with that name for screen
              readers, not repeat it on screen. */}
          <Label htmlFor={NOTES_TEXTAREA_ID} className="sr-only">
            {t.publicBooking.notes}
          </Label>
          <Textarea
            id={NOTES_TEXTAREA_ID}
            placeholder={t.publicBooking.notesPlaceholder}
            rows={3}
            className="mt-2 text-base"
            {...register('client_notes')}
          />
        </>
      )}
    </div>
  );
}
