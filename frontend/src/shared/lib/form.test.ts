import { describe, it, expect, vi } from 'vitest';
/// <reference types="node" />
import type { FieldValues, SubmitHandler, UseFormHandleSubmit } from 'react-hook-form';
import { submitHandler } from './form';

interface DummyFormValues extends FieldValues {
  name: string;
}

function flushMicrotasks(): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, 0));
}

function makeHandleSubmit(): UseFormHandleSubmit<DummyFormValues> {
  return ((onValid: SubmitHandler<DummyFormValues>) => {
    return async (event?: { preventDefault?: () => void }) => {
      event?.preventDefault?.();
      const data: DummyFormValues = { name: 'Ada' };
      await onValid(data, undefined as never);
    };
  }) as UseFormHandleSubmit<DummyFormValues>;
}

describe('submitHandler', () => {
  it('returns a function suitable for React.SubmitEventHandler', () => {
    const handler = submitHandler(makeHandleSubmit(), vi.fn());
    expect(typeof handler).toBe('function');
  });

  it('calls onValid with the resolved form data once the underlying handleSubmit chain runs', async () => {
    const onValid = vi.fn();
    const handler = submitHandler(makeHandleSubmit(), onValid);
    const preventDefault = vi.fn();

    handler({ preventDefault } as unknown as React.SubmitEvent<HTMLFormElement>);
    await flushMicrotasks();

    expect(onValid).toHaveBeenCalledTimes(1);
    expect(onValid).toHaveBeenCalledWith({ name: 'Ada' }, undefined);
  });

  it('calls preventDefault on the passed event via the underlying handleSubmit chain', async () => {
    const handler = submitHandler(makeHandleSubmit(), vi.fn());
    const preventDefault = vi.fn();

    handler({ preventDefault } as unknown as React.SubmitEvent<HTMLFormElement>);
    await flushMicrotasks();

    expect(preventDefault).toHaveBeenCalledTimes(1);
  });

  it('does not throw synchronously when the underlying handleSubmit chain rejects', async () => {
    const onValid = vi.fn().mockRejectedValue(new Error('boom'));
    const handler = submitHandler(makeHandleSubmit(), onValid);
    const preventDefault = vi.fn();

    // The `void` discard means the rejection is deliberately unhandled by design
    // (matches pre-migration `onSubmit={handleSubmit(onSubmit)}` behavior, since React
    // never awaited the returned promise either) — swallow it here purely for test hygiene.
    const swallowRejection = new Promise<void>((resolve) => {
      process.once('unhandledRejection', () => {
        resolve();
      });
    });

    expect(() => {
      handler({ preventDefault } as unknown as React.SubmitEvent<HTMLFormElement>);
    }).not.toThrow();

    await Promise.race([swallowRejection, flushMicrotasks()]);
  });
});
