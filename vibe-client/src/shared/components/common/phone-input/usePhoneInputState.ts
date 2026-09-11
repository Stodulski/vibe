import { useState, useCallback } from 'react';
import { parsePhoneWithPrefix, formatE164 } from '@/shared/lib/phone';
import { DEFAULT_PHONE_PREFIX } from '@/shared/lib/constants';

interface UsePhoneInputStateArgs {
  value: string;
  onChange: (value: string) => void;
}

export function usePhoneInputState({ value, onChange }: UsePhoneInputStateArgs) {
  const [localNumber, setLocalNumber] = useState(() => parsePhoneWithPrefix(value).localNumber);

  // The E.164 value this hook itself last emitted through `onChange`, kept as
  // state (not a ref — the render-phase sync below reads it, and refs cannot
  // be read during render). Typing a space or dash changes `localNumber`
  // (free text) without changing the emitted E.164 string (digits only) —
  // tracking what we emitted lets the sync tell that apart from a real
  // external change to `value` (form reset, prefill), instead of clobbering
  // `localNumber` on every keystroke.
  const [lastEmitted, setLastEmitted] = useState(value);

  // Sync when `value` changes externally (form reset, default values) — a
  // render-phase state adjustment per React's guidance, not an effect.
  const [prevValue, setPrevValue] = useState(value);
  if (value !== prevValue) {
    setPrevValue(value);
    if (value !== lastEmitted) {
      const parsed = parsePhoneWithPrefix(value);
      setLocalNumber(parsed.localNumber);
    }
  }

  const handleLocalChange = useCallback(
    (e: React.ChangeEvent<HTMLInputElement>) => {
      const newLocal = e.target.value;
      setLocalNumber(newLocal);
      const emitted = formatE164(DEFAULT_PHONE_PREFIX, newLocal);
      setLastEmitted(emitted);
      onChange(emitted);
    },
    [onChange],
  );

  // The prefix is fixed to Argentina — no selector changes it — but is still
  // returned for callers/tests that read it off the hook.
  return { prefix: DEFAULT_PHONE_PREFIX, localNumber, handleLocalChange };
}
