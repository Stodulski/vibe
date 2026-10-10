import { afterEach } from 'vitest';
import { cleanup } from '@testing-library/react';
import '@testing-library/jest-dom/vitest';

// Unmount rendered trees after every test. Testing Library mounts into
// document.body; without this, renders pile up and queries match leftovers.
afterEach(() => {
  cleanup();
});
