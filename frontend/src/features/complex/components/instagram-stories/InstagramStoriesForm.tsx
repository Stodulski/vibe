import type { ReactNode } from 'react';
import { Controller, type Control } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { toast } from 'sonner';
import { FormField } from '@/shared/components/common/FormField';
import { SectionFooter } from '@/shared/components/common/SectionFooter';
import { TimeSelect } from '@/shared/components/common/TimeSelect';
import { Label } from '@/shared/components/ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/shared/components/ui/select';
import { Switch } from '@/shared/components/ui/switch';
import { ES_AR } from '@/shared/i18n/es_AR';
import { getProblem } from '@/shared/lib/ApiError';
import { submitHandler, useAppForm } from '@/shared/lib/form';
import { applyServerFieldErrors } from '@/shared/lib/serverFieldErrors';
import { useUpdateInstagramStories } from '../../hooks/useInstagramStories';
import {
  FREED_CAP_OPTIONS,
  instagramStorySettingsFormSchema,
  type InstagramStorySettings,
} from '../../schemas/instagramStories.schema';

const t = ES_AR.instagram.stories;

const FIELDS = ['daily_enabled', 'daily_time', 'freed_enabled', 'freed_daily_cap'] as const;

/** One story type: its switch with the explanation beside it, and the control that tunes it below. */
function StoryToggle({
  id,
  name,
  label,
  help,
  control,
  children,
}: {
  id: string;
  name: 'daily_enabled' | 'freed_enabled';
  label: string;
  help: string;
  control: Control<InstagramStorySettings>;
  children: ReactNode;
}) {
  return (
    <div className="space-y-3">
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0 space-y-1">
          <Label htmlFor={id}>{label}</Label>
          <p className="text-text-tertiary text-xs">{help}</p>
        </div>
        <Controller
          name={name}
          control={control}
          render={({ field }) => <Switch id={id} checked={field.value} onCheckedChange={field.onChange} />}
        />
      </div>
      {children}
    </div>
  );
}

function DailyTimeField({ control, error }: { control: Control<InstagramStorySettings>; error: string | undefined }) {
  return (
    <div className="space-y-1.5">
      <Controller
        name="daily_time"
        control={control}
        render={({ field }) => (
          <TimeSelect value={field.value} onChange={field.onChange} min="08:00" max="22:00" aria-label={t.dailyTime} />
        )}
      />
      {error && (
        <p role="alert" className="text-error-text text-sm">
          {error}
        </p>
      )}
    </div>
  );
}

function FreedCapField({ control, error }: { control: Control<InstagramStorySettings>; error: string | undefined }) {
  return (
    <FormField label={t.freedCap} htmlFor="ig-freed-cap" error={error}>
      <Controller
        name="freed_daily_cap"
        control={control}
        render={({ field }) => (
          <Select
            value={String(field.value)}
            onValueChange={(v) => {
              field.onChange(Number(v));
            }}
          >
            <SelectTrigger id="ig-freed-cap" className="w-full sm:w-48">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {FREED_CAP_OPTIONS.map((n) => (
                <SelectItem key={n} value={String(n)}>
                  {t.capOption(n)}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        )}
      />
    </FormField>
  );
}

/**
 * The two automatic story types. Saving sends all four settings (the PUT
 * replaces them); a 422 lands on the field the server named and the form
 * keeps what the owner typed.
 */
export function InstagramStoriesForm({
  complexId,
  settings,
  preview,
}: {
  complexId: string;
  settings: InstagramStorySettings;
  preview: ReactNode;
}) {
  const update = useUpdateInstagramStories(complexId);
  const form = useAppForm<InstagramStorySettings>({
    resolver: zodResolver(instagramStorySettingsFormSchema),
    defaultValues: settings,
  });
  const { control, formState, reset } = form;

  const onSubmit = (values: InstagramStorySettings) => {
    update.mutate(values, {
      onSuccess: (saved) => {
        reset(saved.settings);
      },
      onError: (error) => {
        applyServerFieldErrors(form, error, { fields: FIELDS, fallback: t.saveError });
        // No problem body (network down): nothing landed on the form to explain it.
        if (!getProblem(error)) toast.error(t.saveError);
      },
    });
  };

  return (
    <form onSubmit={submitHandler(form.handleSubmit, onSubmit)} className="space-y-5" noValidate>
      <StoryToggle id="ig-daily" name="daily_enabled" label={t.daily} help={t.dailyHelp} control={control}>
        <DailyTimeField control={control} error={formState.errors.daily_time?.message} />
        <p className="text-text-tertiary text-xs">{t.dailyStartsNote}</p>
        {preview}
      </StoryToggle>
      <StoryToggle id="ig-freed" name="freed_enabled" label={t.freed} help={t.freedHelp} control={control}>
        <FreedCapField control={control} error={formState.errors.freed_daily_cap?.message} />
      </StoryToggle>
      {formState.errors.root?.message && (
        <p role="alert" className="text-error-text text-sm">
          {formState.errors.root.message}
        </p>
      )}
      <SectionFooter
        align="start"
        submitLabel={t.save}
        pending={update.isPending}
        submitDisabled={!formState.isDirty || update.isPending}
      />
    </form>
  );
}
