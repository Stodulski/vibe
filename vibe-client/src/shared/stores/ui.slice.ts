import type { StateCreator } from 'zustand';
import { STORAGE_KEYS } from '@/shared/lib/storageKeys';
import { safeLocalStorage } from '@/shared/lib/safeStorage';

const SELECTED_COMPLEX_KEY = STORAGE_KEYS.SELECTED_COMPLEX_ID;

export interface UISlice {
  selectedComplexId: string | null;
  setSelectedComplexId: (id: string | null) => void;
}

export const createUISlice: StateCreator<UISlice> = (set) => ({
  selectedComplexId: safeLocalStorage.get(SELECTED_COMPLEX_KEY),
  setSelectedComplexId: (id) => {
    if (id) {
      safeLocalStorage.set(SELECTED_COMPLEX_KEY, id);
    } else {
      safeLocalStorage.remove(SELECTED_COMPLEX_KEY);
    }
    set({ selectedComplexId: id });
  },
});
