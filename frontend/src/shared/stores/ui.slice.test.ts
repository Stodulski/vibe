import { describe, it, expect, beforeEach } from 'vitest';
import type { StoreApi } from 'zustand';
import { createUISlice, type UISlice } from './ui.slice';
import { STORAGE_KEYS } from '@/shared/lib/storageKeys';

function createStore() {
  let state: UISlice = {} as UISlice;
  const setState: StoreApi<UISlice>['setState'] = (partial) => {
    if (typeof partial === 'function') {
      state = { ...state, ...partial(state) };
    } else {
      state = { ...state, ...partial };
    }
  };
  const getState: StoreApi<UISlice>['getState'] = () => state;
  const store = {
    setState,
    getState,
    getInitialState: getState,
    subscribe: () => () => {
      /* noop unsubscribe */
    },
  } satisfies StoreApi<UISlice>;
  const initialState = createUISlice(setState, getState, store);
  state = { ...initialState };
  return {
    getState: () => state,
    ...state,
  };
}

describe('uiSlice', () => {
  beforeEach(() => {
    localStorage.clear();
  });

  it('reads the initial selectedComplexId from localStorage', () => {
    localStorage.setItem(STORAGE_KEYS.SELECTED_COMPLEX_ID, 'complex-from-storage');
    const store = createStore();
    expect(store.getState().selectedComplexId).toBe('complex-from-storage');
  });

  it('setSelectedComplexId persists the value to localStorage', () => {
    const store = createStore();
    store.setSelectedComplexId('complex-123');
    expect(localStorage.getItem(STORAGE_KEYS.SELECTED_COMPLEX_ID)).toBe('complex-123');
  });

  it('setSelectedComplexId(null) removes the key from localStorage', () => {
    const store = createStore();
    store.setSelectedComplexId('complex-123');
    store.setSelectedComplexId(null);
    expect(localStorage.getItem(STORAGE_KEYS.SELECTED_COMPLEX_ID)).toBeNull();
  });

  it('selectedComplexId is null initially', () => {
    const store = createStore();
    expect(store.getState().selectedComplexId).toBeNull();
  });

  it('setSelectedComplexId updates the selected complex', () => {
    const store = createStore();
    store.setSelectedComplexId('complex-123');
    expect(store.getState().selectedComplexId).toBe('complex-123');
  });

  it('setSelectedComplexId can set to null', () => {
    const store = createStore();
    store.setSelectedComplexId('complex-123');
    store.setSelectedComplexId(null);
    expect(store.getState().selectedComplexId).toBeNull();
  });
});
