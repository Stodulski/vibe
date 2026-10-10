import { describe, it, expect } from 'vitest';
import { renderHook, act } from '@testing-library/react';
import type { ChangeEvent } from 'react';
import { usePhoneInputState } from './usePhoneInputState';

// A minimal controlled-input harness: `onChange` writes to a ref that becomes
// the next `value` prop, exactly like a parent form field would, so the
// hook's own emitted value flows back in through props on the next render —
// this is what exercises the external-vs-self-emitted distinction in A3.
function renderControlled(initial: string) {
  let value = initial;
  const rendered = renderHook(
    (props: { value: string }) =>
      usePhoneInputState({
        value: props.value,
        onChange: (v) => {
          value = v;
        },
      }),
    { initialProps: { value: initial } },
  );

  const type = (e: { target: { value: string } }) => {
    act(() => {
      rendered.result.current.handleLocalChange(e as unknown as ChangeEvent<HTMLInputElement>);
    });
    rendered.rerender({ value });
  };

  return { ...rendered, type, getEmitted: () => value };
}

describe('usePhoneInputState', () => {
  it('keeps a typed space in the local value and emits digits-only E.164', () => {
    const { result, type, getEmitted } = renderControlled('+54');

    type({ target: { value: '11 2345' } });

    expect(result.current.localNumber).toBe('11 2345');
    expect(getEmitted()).toBe('+54112345');
  });

  it('clears the field when the value is externally reset to empty', () => {
    const { result, type, rerender } = renderControlled('+54');

    type({ target: { value: '11 2345' } });
    expect(result.current.localNumber).toBe('11 2345');

    rerender({ value: '' });

    expect(result.current.localNumber).toBe('');
  });

  it('populates the field from an external prefill', () => {
    const { result, rerender } = renderControlled('');

    rerender({ value: '+5491155551234' });

    expect(result.current.prefix).toBe('+54');
    expect(result.current.localNumber).toBe('91155551234');
  });

  // The prefix is fixed to Argentina — there is no selector to drive it, and
  // no `defaultPrefix`/`onPrefixChange` args exist anymore. Typing a bare
  // local number, starting from an empty field, must still emit `+54`.
  it('always prepends +54 to a typed local number, with no prefix input at all', () => {
    const { result, type, getEmitted } = renderControlled('');

    type({ target: { value: '1123456789' } });

    expect(result.current.prefix).toBe('+54');
    expect(getEmitted()).toBe('+541123456789');
  });
});
