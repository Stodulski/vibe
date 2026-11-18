import { useId, useState } from 'react';
import {
  AlertDialog,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogTitle,
  Button,
  FormField,
  Input,
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@vibe/ui';
import type { WaitlistTarget } from '../../api/types';
import { useChallengedSubmit } from '../../hooks/useChallengedSubmit';
import { useJoinPlaceWaitlist } from '../../hooks/useJoinPlaceWaitlist';
import { emailField } from '../../lib/validations';
import { BOOKING_MESSAGES } from '../../messages';
import { useHoldChallenge } from '../../pages/complex-page/useHoldChallenge';
import { TurnstileField } from '../booking-form/TurnstileField';
import { waitlistRefusal } from './waitlistRefusal';

const t = BOOKING_MESSAGES.waitlist;


/** One taken slot the visitor can pick in the booking's "¿No está el horario que buscás?". */
export interface WaitlistSlotOption {
  value: string;
  label: string;
  target: WaitlistTarget;
}

/** What the popup asks for: one known place, or a pick among taken slots. */
export interface WaitlistRequest {
  title: string;
  description: string;
  submitLabel: string;
  /** The place, when it is already known (a full event). */
  target?: WaitlistTarget | undefined;
  /** The taken slots to pick from, when the place is not known yet (a booking). */
  slots?: readonly WaitlistSlotOption[] | undefined;
}

interface WaitlistDialogProps {
  slug: string;
  /** The open request, or `null` while the popup is closed. */
  request: WaitlistRequest | null;
  onClose: () => void;
}

/**
 * The "Avisarme" popup: an email field, a select of taken slots when the place is
 * not known yet, and a send button. On success it says the email was saved.
 */
export function WaitlistDialog({ slug, request, onClose }: WaitlistDialogProps) {
  return (
    <AlertDialog
      open={request !== null}
      onOpenChange={(open) => {
        if (!open) onClose();
      }}
    >
      <AlertDialogContent className="bg-bg-subtle border-border-subtle flex flex-col gap-5 rounded-2xl border p-6 shadow-lg sm:max-w-md">
        {/* Keyed by the request so each opening starts with an empty form. */}
        {request && <WaitlistForm key={request.title + (request.target ? JSON.stringify(request.target) : '')} slug={slug} request={request} onClose={onClose} />}
      </AlertDialogContent>
    </AlertDialog>
  );
}

function WaitlistForm({ slug, request, onClose }: { slug: string; request: WaitlistRequest; onClose: () => void }) {
  const id = useId();
  const [email, setEmail] = useState('');
  const [slot, setSlot] = useState('');
  const [problem, setProblem] = useState<{ field: 'slot' | 'email'; message: string } | null>(null);
  const join = useJoinPlaceWaitlist(slug);
  const { challenge, generation, onTokenChange } = useHoldChallenge();
  const send = useChallengedSubmit<{ target: WaitlistTarget; email: string }>(challenge, (data, token) => {
    join.mutate({ ...data, token }, { onSettled: challenge.reset });
  });
  const slots = request.slots;

  if (join.isSuccess) return <WaitlistDone title={request.title} onClose={onClose} />;

  const submit = () => {
    const parsed = emailField.safeParse(email);
    const target = slots ? slots.find((option) => option.value === slot)?.target : request.target;
    if (slots && !target) {
      setProblem({ field: 'slot', message: t.slotRequired });
      return;
    }
    if (!parsed.success) {
      setProblem({ field: 'email', message: t.emailInvalid });
      return;
    }
    if (!target) return;
    setProblem(null);
    send({ target, email: parsed.data });
  };

  return (
    <form
      noValidate
      className="flex flex-col gap-5"
      onSubmit={(event) => {
        event.preventDefault();
        submit();
      }}
    >
      <div className="flex flex-col gap-1">
        <AlertDialogTitle className="text-text-primary text-xl leading-tight font-bold">{request.title}</AlertDialogTitle>
        <AlertDialogDescription className="text-text-secondary text-sm">{request.description}</AlertDialogDescription>
      </div>
      {slots && <SlotSelect id={`${id}-slot`} slots={slots} value={slot} onChange={setSlot} error={problem?.field === 'slot' ? problem.message : undefined} />}
      <FormField label={t.emailLabel} htmlFor={`${id}-email`} error={problem?.field === 'email' ? problem.message : join.isError ? waitlistRefusal(join.error) : undefined}>
        <Input
          id={`${id}-email`}
          type="email"
          inputMode="email"
          autoComplete="email"
          value={email}
          onChange={(event) => {
            setEmail(event.target.value);
          }}
          className="h-12 text-base"
        />
      </FormField>
      <TurnstileField key={generation} onTokenChange={onTokenChange} />
      <FormActions pending={join.isPending} submitLabel={request.submitLabel} onClose={onClose} />
    </form>
  );
}

function WaitlistDone({ title, onClose }: { title: string; onClose: () => void }) {
  return (
    <>
      <AlertDialogTitle className="text-text-primary text-xl leading-tight font-bold">{title}</AlertDialogTitle>
      <AlertDialogDescription className="text-text-secondary text-sm">{t.done}</AlertDialogDescription>
      <Button size="lg" className="min-h-12 w-full sm:w-auto" onClick={onClose}>
        {t.close}
      </Button>
    </>
  );
}

/** The taken slots to wait for, as the app's Select. */
function SlotSelect({
  id,
  slots,
  value,
  onChange,
  error,
}: {
  id: string;
  slots: readonly WaitlistSlotOption[];
  value: string;
  onChange: (value: string) => void;
  error: string | undefined;
}) {
  return (
    <FormField label={t.slotLabel} htmlFor={id} error={error}>
      <Select value={value} onValueChange={onChange}>
        <SelectTrigger id={id} className="h-12 w-full">
          <SelectValue placeholder={t.slotPlaceholder} />
        </SelectTrigger>
        <SelectContent>
          {slots.map((option) => (
            <SelectItem key={option.value} value={option.value}>
              {option.label}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    </FormField>
  );
}

function FormActions({ pending, submitLabel, onClose }: { pending: boolean; submitLabel: string; onClose: () => void }) {
  return (
    <div className="flex flex-col gap-2 sm:flex-row">
      <Button type="submit" size="lg" className="min-h-12 w-full sm:w-auto" disabled={pending}>
        {pending ? t.sending : submitLabel}
      </Button>
      <Button type="button" variant="outline" size="lg" className="min-h-12 w-full sm:w-auto" onClick={onClose}>
        {t.cancel}
      </Button>
    </div>
  );
}
