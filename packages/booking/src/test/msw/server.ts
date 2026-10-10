import { setupServer } from 'msw/node';
import { handlers } from './handlers';

/**
 * The single MSW server of the booking suite. Started once in `../setup.ts`;
 * tests layer scenario handlers on top with `server.use(...)`.
 */
export const server = setupServer(...handlers);
