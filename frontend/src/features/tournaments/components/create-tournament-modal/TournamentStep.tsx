import { type UseFormReturn } from 'react-hook-form';
import { SectionFooter } from '@/shared/components/common/SectionFooter';
import { ES_AR } from '@/shared/i18n/es_AR';
import { TournamentBasicsFields } from './TournamentBasicsFields';
import type { TournamentFormValues } from '../../schemas/tournamentForm.schema';

interface Props {
  form: UseFormReturn<TournamentFormValues>;
  /** The server's refusal, shown when it sent the owner back here. */
  error: string | undefined;
  onCancel: () => void;
  onNext: () => void;
}

/** Step 1: name, sport and the first and last day. */
export function TournamentStep({ form, error, onCancel, onNext }: Props) {
  return (
    <>
      <TournamentBasicsFields form={form} />
      {error && (
        <p role="alert" className="text-error-text text-sm">
          {error}
        </p>
      )}
      <SectionFooter onCancel={onCancel} submitLabel={ES_AR.common.next} onSubmit={onNext} />
    </>
  );
}
