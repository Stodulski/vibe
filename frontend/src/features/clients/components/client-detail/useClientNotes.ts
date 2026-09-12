import { useState, useRef, useEffect } from 'react';
import { toast } from 'sonner';
import { ES_AR } from '@/shared/i18n/es_AR';
import { getHttpErrorMessage } from '@/shared/lib/utils';
import { useUpdateClient } from '../../hooks/useUpdateClient';
import type { Client } from '@/shared/types/api.types';

const t = ES_AR;
const AUTOSAVE_DELAY_MS = 800;

export function useClientNotes(complexId: string, client: Client | null | undefined) {
  const updateClient = useUpdateClient(complexId);
  // Initial notes come directly from the first `client` seen (mirrors the
  // original mount-time effect run).
  const [notes, setNotes] = useState(() => client?.notes ?? '');
  const [saveError, setSaveError] = useState<string | undefined>(undefined);
  const debounceRef = useRef<ReturnType<typeof setTimeout>>(undefined);
  const lastAttemptRef = useRef<{ value: string; clientId: string } | undefined>(undefined);

  // Adjust local notes state only when a different CLIENT arrives (a new
  // client selected). Comparing `client.id` instead of the object reference
  // matters: `useUpdateClient`'s onSuccess invalidates `clients.detail`,
  // which refetches and hands back a new `client` reference for the SAME
  // client. Resetting on every reference would overwrite whatever the
  // person is typing mid-debounce with the (now stale) server value.
  const [prevClientId, setPrevClientId] = useState(client?.id);
  if (client?.id !== prevClientId) {
    setPrevClientId(client?.id);
    setSaveError(undefined);
    if (client) setNotes(client.notes ?? '');
  }

  // Drops any pending autosave when switching clients (or unmounting) —
  // touches the ref, so it belongs in an effect, not the render-phase
  // adjustment above.
  useEffect(
    () => () => {
      clearTimeout(debounceRef.current);
    },
    [client?.id],
  );

  const saveNotes = (value: string, clientId: string) => {
    lastAttemptRef.current = { value, clientId };
    setSaveError(undefined);
    updateClient.mutate(
      { clientId, data: { notes: value } },
      {
        onSuccess: () => {
          lastAttemptRef.current = undefined;
          toast.success(t.clients.notesUpdated);
        },
        // Keep the typed draft as-is (it already lives in `notes`, set by
        // `onNotesChange` below) and surface an inline, retryable error
        // instead of silently discarding the failed save.
        onError: (error) => {
          setSaveError(getHttpErrorMessage(error, t.clients.notesSaveError));
        },
      },
    );
  };

  const onNotesChange = (value: string) => {
    setNotes(value);
    if (!client) return;
    clearTimeout(debounceRef.current);
    debounceRef.current = setTimeout(() => {
      saveNotes(value, client.id);
    }, AUTOSAVE_DELAY_MS);
  };

  const retrySave = () => {
    const attempt = lastAttemptRef.current;
    if (attempt) saveNotes(attempt.value, attempt.clientId);
  };

  return {
    notes,
    isSaving: updateClient.isPending,
    saveError,
    onNotesChange,
    retrySave,
  };
}
