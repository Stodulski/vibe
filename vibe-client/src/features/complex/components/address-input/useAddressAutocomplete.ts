import { useState, useRef, useEffect, useCallback } from 'react';
import { fetchAutocompletePredictions, fetchPlaceDetails } from './addressApi';
import { generateSessionToken, type AddressSelection, type Prediction } from './types';
import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;

function isAbortError(err: unknown): boolean {
  return err instanceof DOMException && err.name === 'AbortError';
}

function useAddressSearch() {
  const [predictions, setPredictions] = useState<Prediction[]>([]);
  const [open, setOpen] = useState(false);
  const [activeIndex, setActiveIndex] = useState(-1);
  const [error, setError] = useState<string | null>(null);
  const debounceRef = useRef<ReturnType<typeof setTimeout>>(null);
  const abortRef = useRef<AbortController | null>(null);

  const search = useCallback((query: string, sessionToken: string) => {
    if (debounceRef.current) clearTimeout(debounceRef.current);
    // A new keystroke supersedes whatever request is still in flight for the
    // previous one — without this, a slow response for "Av. Lib" can land
    // after a faster one for "Av. Libertador" and overwrite it with stale
    // predictions.
    abortRef.current?.abort();
    setError(null);

    if (query.length < 3) {
      setPredictions([]);
      setOpen(false);
      return;
    }

    debounceRef.current = setTimeout(() => {
      const controller = new AbortController();
      abortRef.current = controller;
      void (async () => {
        try {
          const results = await fetchAutocompletePredictions(query, sessionToken, controller.signal);
          // Belt and suspenders alongside the abort itself: a mocked/cached
          // transport can still resolve after being superseded, and a
          // superseded response must never overwrite what the user is now
          // seeing.
          if (controller.signal.aborted) return;
          setPredictions(results);
          setOpen(results.length > 0);
          setActiveIndex(-1);
        } catch (err) {
          if (isAbortError(err) || controller.signal.aborted) return;
          setError(t.complex.addressSearchError);
        }
      })();
    }, 500);
  }, []);

  useEffect(() => {
    // Cleanup on unmount: an in-flight debounce timer or request must not
    // call `setState` on a component that no longer exists.
    return () => {
      if (debounceRef.current) clearTimeout(debounceRef.current);
      abortRef.current?.abort();
    };
  }, []);

  return { predictions, setPredictions, open, setOpen, activeIndex, setActiveIndex, search, error };
}

interface UsePlaceSelectionArgs {
  onChange: (value: string) => void;
  onSelect: (details: AddressSelection) => void;
  clearPredictions: () => void;
  closeList: () => void;
}

function usePlaceSelection({ onChange, onSelect, clearPredictions, closeList }: UsePlaceSelectionArgs) {
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const sessionTokenRef = useRef(generateSessionToken());
  const abortRef = useRef<AbortController | null>(null);

  useEffect(() => {
    return () => {
      abortRef.current?.abort();
    };
  }, []);

  const handleSelect = async (prediction: Prediction) => {
    closeList();
    clearPredictions();
    onChange(prediction.description);
    setLoading(true);
    setError(null);

    abortRef.current?.abort();
    const controller = new AbortController();
    abortRef.current = controller;

    try {
      const details = await fetchPlaceDetails(prediction.place_id, sessionTokenRef.current, controller.signal);
      if (controller.signal.aborted) return;
      sessionTokenRef.current = generateSessionToken();

      onSelect({
        address: details.address,
        city: details.city,
        province: details.province,
        formatted_address: details.formatted_address,
        latitude: parseFloat(details.latitude),
        longitude: parseFloat(details.longitude),
      });
    } catch (err) {
      // Aborted by a newer selection or by unmount: not a user-facing error.
      if (isAbortError(err) || controller.signal.aborted) return;
      // Keep the text the user selected, even though details failed — but
      // say so, instead of leaving them looking at a spinner that vanished
      // and a submit that will fail later with "dirección obligatoria".
      setError(t.complex.addressDetailsError);
    } finally {
      if (!controller.signal.aborted) setLoading(false);
    }
  };

  return { loading, error, sessionTokenRef, handleSelect };
}

function buildComboboxKeyDownHandler(
  open: boolean,
  predictions: Prediction[],
  activeIndex: number,
  setActiveIndex: (fn: (i: number) => number) => void,
  setOpen: (open: boolean) => void,
  handleSelect: (prediction: Prediction) => void,
) {
  return (e: React.KeyboardEvent) => {
    if (!open || predictions.length === 0) return;

    if (e.key === 'ArrowDown') {
      e.preventDefault();
      setActiveIndex((i) => (i < predictions.length - 1 ? i + 1 : 0));
    } else if (e.key === 'ArrowUp') {
      e.preventDefault();
      setActiveIndex((i) => (i > 0 ? i - 1 : predictions.length - 1));
    } else if (e.key === 'Enter' && activeIndex >= 0) {
      e.preventDefault();
      const selected = predictions[activeIndex];
      if (selected) handleSelect(selected);
    } else if (e.key === 'Escape') {
      setOpen(false);
    }
  };
}

function useCloseOnOutsideClick(
  containerRef: React.RefObject<HTMLDivElement | null>,
  setOpen: (open: boolean) => void,
) {
  useEffect(() => {
    function handleClick(e: MouseEvent) {
      if (containerRef.current && !containerRef.current.contains(e.target as Node)) {
        setOpen(false);
      }
    }
    document.addEventListener('mousedown', handleClick);
    return () => {
      document.removeEventListener('mousedown', handleClick);
    };
  }, [containerRef, setOpen]);
}

interface UseAddressAutocompleteArgs {
  confirmed: boolean;
  onChange: (value: string) => void;
  onSelect: (details: AddressSelection) => void;
  onClear: () => void;
}

export function useAddressAutocomplete({ confirmed, onChange, onSelect, onClear }: UseAddressAutocompleteArgs) {
  const {
    predictions,
    setPredictions,
    open,
    setOpen,
    activeIndex,
    setActiveIndex,
    search,
    error: searchError,
  } = useAddressSearch();
  const containerRef = useRef<HTMLDivElement>(null);
  const {
    loading,
    error: selectionError,
    sessionTokenRef,
    handleSelect,
  } = usePlaceSelection({
    onChange,
    onSelect,
    clearPredictions: () => {
      setPredictions([]);
    },
    closeList: () => {
      setOpen(false);
    },
  });

  const handleKeyDown = buildComboboxKeyDownHandler(open, predictions, activeIndex, setActiveIndex, setOpen, (p) => {
    void handleSelect(p);
  });

  useCloseOnOutsideClick(containerRef, setOpen);

  const handleInputChange = (newValue: string) => {
    onChange(newValue);
    // If the user edits after selecting, invalidate the selection.
    if (confirmed) {
      onClear();
    }
    search(newValue, sessionTokenRef.current);
  };

  return {
    predictions,
    open,
    activeIndex,
    loading,
    error: searchError ?? selectionError,
    containerRef,
    handleSelect,
    handleKeyDown,
    handleInputChange,
    setOpen,
    setActiveIndex,
  };
}
