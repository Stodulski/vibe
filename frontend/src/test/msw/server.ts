import { setupServer } from 'msw/node';
import { handlers } from './handlers';

/**
 * The single MSW server instance for the unit suite. Started once in
 * `src/test/setup.ts`; individual tests layer scenario-specific handlers on
 * top with `server.use(...)`, which `afterEach`'s `server.resetHandlers()`
 * peels back off to the defaults in `./handlers`.
 */
export const server = setupServer(...handlers);
