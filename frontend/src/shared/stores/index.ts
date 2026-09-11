import { create } from 'zustand';
import { createAuthSlice, type AuthSlice } from './auth.slice';
import { createUISlice, type UISlice } from './ui.slice';

export type AppStore = AuthSlice & UISlice;

export const useStore = create<AppStore>()((...a) => ({
  ...createAuthSlice(...a),
  ...createUISlice(...a),
}));
