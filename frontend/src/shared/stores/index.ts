import { create } from 'zustand';
import { createAuthSlice, type AuthSlice } from './auth.slice';

export type AppStore = AuthSlice;

export const useStore = create<AppStore>()((...a) => ({
  ...createAuthSlice(...a),
}));
