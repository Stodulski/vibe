import type { FieldValues, SubmitHandler, UseFormHandleSubmit } from 'react-hook-form';

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
