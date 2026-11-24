import api, { withSignal } from '@/shared/lib/ky';
import type {
  Court,
  CourtWithPrices,
  CourtPrice,
  CreateCourtRequest,
  UpdateCourtRequest,
  UpdatePricesRequest,
  BlockedSlot,
  FixedBlock,
  EventBlock,
  MatchBlock,
  BlockSlotRequest,
} from '@/shared/types/api.types';
import { parseWith } from '@/shared/lib/apiParse';
import { listCourts } from '@/shared/api/courts.api';
import { messageResponseSchema } from '@/shared/schemas/envelope.schema';
import {
  courtEnvelopeSchema,
  pricesEnvelopeSchema,
  blockedSlotEnvelopeSchema,
  blockedSlotsEnvelopeSchema,
} from '@/shared/schemas/court.schema';

export const courtsApi = {
  // Delegates to the shared one: see `shared/api/courts.api.ts`.
  list: (complexId: string, signal?: AbortSignal): Promise<{ courts: CourtWithPrices[] }> =>
    listCourts(complexId, signal),

  create: (complexId: string, data: CreateCourtRequest): Promise<{ court: Court }> =>
    api
      .post(`complexes/${complexId}/courts`, { json: data })
      .json()
      .then(parseWith(courtEnvelopeSchema, 'courtsApi.create')),

  update: (complexId: string, courtId: string, data: UpdateCourtRequest): Promise<{ court: Court }> =>
    api
      .put(`complexes/${complexId}/courts/${courtId}`, { json: data })
      .json()
      .then(parseWith(courtEnvelopeSchema, 'courtsApi.update')),

  delete: (complexId: string, courtId: string): Promise<{ message: string }> =>
    api
      .delete(`complexes/${complexId}/courts/${courtId}`)
      .json()
      .then(parseWith(messageResponseSchema, 'courtsApi.delete')),

  updatePrices: (complexId: string, courtId: string, data: UpdatePricesRequest): Promise<{ prices: CourtPrice[] }> =>
    api
      .put(`complexes/${complexId}/courts/${courtId}/prices`, { json: data })
      .json()
      .then(parseWith(pricesEnvelopeSchema, 'courtsApi.updatePrices')),

  blockSlot: (complexId: string, courtId: string, data: BlockSlotRequest): Promise<{ blocked_slot: BlockedSlot }> =>
    api
      .post(`complexes/${complexId}/courts/${courtId}/block`, { json: data })
      .json()
      .then(parseWith(blockedSlotEnvelopeSchema, 'courtsApi.blockSlot')),

  listBlockedSlots: (
    complexId: string,
    dateFrom: string,
    dateTo: string,
    signal?: AbortSignal,
  ): Promise<{
    blocked_slots: BlockedSlot[];
    fixed_blocks: FixedBlock[];
    event_blocks: EventBlock[];
    match_blocks: MatchBlock[];
  }> =>
    api
      .get(`complexes/${complexId}/blocked-slots`, {
        searchParams: { date_from: dateFrom, date_to: dateTo },
        ...withSignal(signal),
      })
      .json()
      .then(parseWith(blockedSlotsEnvelopeSchema, 'courtsApi.listBlockedSlots')),

  deleteBlockedSlot: (complexId: string, slotId: string): Promise<{ message: string }> =>
    api
      .delete(`complexes/${complexId}/blocked-slots/${slotId}`)
      .json()
      .then(parseWith(messageResponseSchema, 'courtsApi.deleteBlockedSlot')),
};
