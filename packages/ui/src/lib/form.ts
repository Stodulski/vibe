import { useForm } from 'react-hook-form';
import type { FieldValues, SubmitHandler, UseFormHandleSubmit, UseFormProps, UseFormReturn } from 'react-hook-form';

/**
 * `useForm` with this app's validation timing already chosen.
 *
 * react-hook-form validates on submit by default, so a mistyped email was
 * only ever reported once the whole form had been filled in and sent — the
 * person is told about the second field while looking at the last one, and has
 * to walk back up the form to find it. `'onBlur'` reports it when they leave
 * the field, which is the moment they are still thinking about that field and
 * have not yet invested in the rest.
 *
 * Not `'onChange'`: complaining about `jua` while someone is still typing
 * `juan@…` is noise, and it is the one thing that makes inline validation feel
 * hostile. A form with a genuine reason to validate per keystroke (the blocked
 * slot modal, whose submit button is enabled from the form's own validity)
 * passes `mode` explicitly and that wins — the default is spread over, not
 * enforced.
 */
export function useAppForm<TFieldValues extends FieldValues, TContext = unknown, TTransformedValues = TFieldValues>(
  props: UseFormProps<TFieldValues, TContext, TTransformedValues>,
): UseFormReturn<TFieldValues, TContext, TTransformedValues> {
  return useForm<TFieldValues, TContext, TTransformedValues>({ mode: 'onBlur', ...props });
}

/**
 * Wraps a react-hook-form `handleSubmit` so it can be passed directly to
 * `<form onSubmit={...}>` without triggering `@typescript-eslint/no-confusing-void-expression`.
 *
 * `handleSubmit(onValid)` returns an async function; React's `onSubmit` prop expects a
 * `void`-returning handler. This wraps the call in a block body and discards the returned
 * promise with a single, audited `void` — no per-call-site `void` wrappers needed.
 *
 * Deviation from design: the design's literal interface used `React.FormEventHandler<T>`,
 * but that type is marked `@deprecated` by the installed React 19 types (it no longer matches
 * the real `onSubmit` prop type) and trips `@typescript-eslint/no-deprecated`. `SubmitEvent<T>`
 * is a subtype of `SyntheticEvent`/`BaseSyntheticEvent`, so `React.SubmitEventHandler<T>` is
 * both the currently-correct type for `<form onSubmit>` and structurally compatible with
 * react-hook-form's `handleSubmit` return signature (`(e?: React.BaseSyntheticEvent) => ...`).
 */
export function submitHandler<T extends FieldValues>(
  handleSubmit: UseFormHandleSubmit<T>,
  onValid: SubmitHandler<T>,
): React.SubmitEventHandler<HTMLFormElement> {
  const run = handleSubmit(onValid);
  return (event) => {
    void run(event);
  };
}
