export function useTheme() {
  return {
    theme: 'dark' as const,
    toggleTheme: () => {
      /* dark mode only — no toggle yet */
    },
  };
}
