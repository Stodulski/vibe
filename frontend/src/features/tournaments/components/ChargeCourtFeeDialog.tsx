import { useState } from 'react';
import { toast } from 'sonner';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/shared/components/common/AppDialog';
import { FormField } from '@/shared/components/common/FormField';
import { SectionFooter } from '@/shared/components/common/SectionFooter';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/shared/components/ui/select';
import { ES_AR } from '@/shared/i18n/es_AR';
import { formatPrice } from '@/shared/lib/utils';
import { useChargeCourtFee, type CourtFeeTarget } from '../hooks/useTournamentChanges';
import { courtFeeErrorMessage } from '../lib/tournamentErrors';

const t = ES_AR.tournaments.courtFee;

/** The methods a court fee is collected with at the complex: the same two a booking is charged with at the venue. */
const METHODS = ['cash', 'transfer'] as const;
type Method = (typeof METHODS)[number];

interface ChargeCourtFeeDialogProps {
  open: boolean;
  onClose: () => void;
  complexId: string;
  tournamentId: string;
  /** The pair (a side of a match) being charged; null while closed. */
  target: CourtFeeTarget | null;
  /** The pair's name and what it owes. */
  pairName: string;
  amount: number;
}

/** Collects a pair's court fee by cash or transfer; the money goes into the open cash session. Mounted only while open. */
export function ChargeCourtFeeDialog({ open, onClose, target, ...rest }: ChargeCourtFeeDialogProps) {
  return (
    <Dialog
      open={open}
      onOpenChange={(v) => {
        if (!v) onClose();
      }}
    >
      <DialogContent className="sm:max-w-md">
        {open && target && <ChargeBody target={target} onClose={onClose} {...rest} />}
      </DialogContent>
    </Dialog>
  );
}

type BodyProps = Omit<ChargeCourtFeeDialogProps, 'open' | 'target'> & { target: CourtFeeTarget };

function ChargeBody({ onClose, complexId, tournamentId, target, pairName, amount }: BodyProps) {
  const [method, setMethod] = useState<Method>('cash');
  const charge = useChargeCourtFee(complexId, tournamentId);

  const submit = () => {
    charge.mutate(
      { ...target, method },
      {
        onSuccess: () => {
          toast.success(t.chargeSuccess);
          onClose();
        },
        onError: (error) => {
          toast.error(t.errors.chargeUnknown, { description: courtFeeErrorMessage(error, 'charge') });
        },
      },
    );
  };

  return (
    <>
      <DialogHeader>
        <DialogTitle>{t.chargeTitle}</DialogTitle>
        <DialogDescription>{t.chargeDescription(pairName, formatPrice(amount))}</DialogDescription>
      </DialogHeader>
      <FormField label={t.method} htmlFor="court-fee-method">
        <Select
          value={method}
          onValueChange={(value) => {
            setMethod(value === 'transfer' ? 'transfer' : 'cash');
          }}
        >
          <SelectTrigger id="court-fee-method">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {METHODS.map((value) => (
              <SelectItem key={value} value={value}>
                {ES_AR.bookings.paymentMethods[value]}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </FormField>
      <SectionFooter
        onCancel={onClose}
        cancelDisabled={charge.isPending}
        submitLabel={t.chargeSubmit(formatPrice(amount))}
        pending={charge.isPending}
        onSubmit={submit}
      />
    </>
  );
}
