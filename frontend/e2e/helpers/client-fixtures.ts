import { getNextWeekday } from './test-data';
import type { getSharedSetup } from './shared-setup';

/**
 * Seed the two standard E2E test clients (`ClienteUno`, `ClienteDos`) via
 * booking creation, tolerating pre-existing/failed creation (the shared E2E
 * DB may already have them from a prior run). Returns whether at least one
 * creation call is known to have succeeded this run.
 *
 * Extracted from `clients.spec.ts`'s `beforeAll` (slice 10, max-lines
 * decomposition) — shared by `clients.spec.ts` and `clients-search.spec.ts`
 * so both suites seed identical fixture data.
 */
export async function createTestClients(setup: Awaited<ReturnType<typeof getSharedSetup>>): Promise<boolean> {
  let hasClients = false;
  const nextWeekday = getNextWeekday();

  try {
    // No `end_time`: the real API computes it server-side from `start_time`
    // + `duration_minutes` and rejects an explicit `end_time` key ("body
    // contains unknown key") -- see ApiHelper.createBooking's comment. Both
    // calls used to pass one, so both silently failed every run (caught
    // below) and this fixture never actually seeded ClienteUno/ClienteDos.
    await setup.api.createBooking(setup.complexId, setup.courtId, {
      date: nextWeekday,
      start_time: '08:00',
      duration_minutes: 90,
      client_first_name: 'ClienteUno',
      client_last_name: 'TestE2E',
      client_phone: '+5491100000033',
      client_email: 'clienteuno@test.com',
    });
    hasClients = true;
  } catch {
    /* may already exist or fail */
  }

  try {
    await setup.api.createBooking(setup.complexId, setup.courtId, {
      date: nextWeekday,
      start_time: '11:30',
      duration_minutes: 90,
      client_first_name: 'ClienteDos',
      client_last_name: 'TestE2E',
      client_phone: '+5491100000034',
      client_email: 'clientedos@test.com',
    });
    hasClients = true;
  } catch {
    /* may already exist or fail */
  }

  return hasClients;
}
