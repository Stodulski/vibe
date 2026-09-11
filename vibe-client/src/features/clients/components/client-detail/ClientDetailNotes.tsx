import { useId, type CSSProperties } from 'react';
import { Button } from '@/shared/components/ui/button';
import { Textarea } from '@/shared/components/ui/textarea';
import { FormField } from '@/shared/components/common/FormField';
import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;

interface ClientDetailNotesProps {
  notes: string;
  isSaving: boolean;
  saveError?: string | undefined;
  onChange: (value: string) => void;
  onRetry: () => void;
}

export function ClientDetailNotes({ notes, isSaving, saveError, onChange, onRetry }: ClientDetailNotesProps) {
  const fieldId = useId();
  const errorId = saveError ? `${fieldId}-error` : undefined;

  return (
    <FormField
      className="space-y-2"
      htmlFor={fieldId}
      label={t.clients.notes}
      error={saveError}
      labelSuffix={
        isSaving ? (
          <span className="text-micro text-text-tertiary">{t.common.saving}</span>
        ) : saveError ? (
          <Button type="button" variant="link" size="xs" className="h-auto p-0" onClick={onRetry}>
            {t.clients.retry}
          </Button>
        ) : undefined
      }
    >
      <Textarea
        id={fieldId}
        value={notes}
        onChange={(e) => {
          onChange(e.target.value);
        }}
        placeholder={t.clients.notesPlaceholder}
        rows={3}
        aria-invalid={!!saveError}
        aria-describedby={errorId}
        // The touch-target rule in globals.css sets a min-height on every
        // textarea that outranks a utility class, so the three-line floor
        // travels as a custom property it reads instead.
        style={{ '--textarea-min-height': '6rem' } as CSSProperties}
        className="min-h-24 resize-none"
      />
    </FormField>
  );
}
