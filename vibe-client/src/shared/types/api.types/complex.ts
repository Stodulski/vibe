// ─── Complex ───

/**
 * What a venue offers. A closed vocabulary — the server's complexes_amenities_known CHECK has the
 * matching CHECK constraint, and the union is what keeps a typo here from
 * becoming a 500 there.
 */
export type Amenity =
  | 'parking'
  | 'changing_rooms'
  | 'showers'
  | 'bar'
  | 'racket_rental'
  | 'pro_shop'
  | 'wifi'
  | 'lockers'
  | 'lessons'
  | 'tournaments'
  | 'accessible'
  | 'match_recording';

export interface Complex {
  id: string;
  owner_id: string;
  name: string;
  slug: string;
  address: string;
  city: string;
  province: string;
  country_code: string;
  currency: string;
  phone: string;
  email: string | null;
  logo_url: string | null;
  cover_url: string | null;
  deposit_percentage: number;
  cancellation_hours: number;
  latitude: number | null;
  longitude: number | null;
  is_active: boolean;
  amenities: Amenity[];
  /**
   * How many courts this complex has — present only where the server counted
   * them, which today is the owner's list.
   *
   * Optional on purpose: absent means "not counted", zero means "no courts".
   * A screen that cannot tell those apart would announce a brand new venue as
   * broken every time it loaded a complex from somewhere else.
   */
  court_count?: number;
  /**
   * Whether this venue can take an online payment.
   *
   * Prefer this over `mp_user_id` for anything the UI decides. The id answers
   * "which MercadoPago account is linked" and was read three times as "is this
   * complex OK?" — the selector warned about it, onboarding called such a
   * complex incomplete, and the manual booking form refused to save. A club
   * taking cash lost its booking form to that confusion.
   */
  payments_enabled: boolean;
  mp_user_id?: string | null;
  created_at: string;
  updated_at: string;
}

export type DayOfWeek = 'monday' | 'tuesday' | 'wednesday' | 'thursday' | 'friday' | 'saturday' | 'sunday';

export interface Schedule {
  id: string;
  complex_id: string;
  day: DayOfWeek;
  open_time: string;
  close_time: string;
  is_closed: boolean;
}

export interface CreateComplexRequest {
  name: string;
  slug: string;
  address: string;
  city: string;
  province: string;
  phone: string;
  email?: string;
  deposit_percentage: number;
  cancellation_hours: number;
  latitude?: number;
  longitude?: number;
}

export type UpdateComplexRequest = Partial<CreateComplexRequest> & {
  logo_url?: string;
  cover_url?: string;
  is_active?: boolean;
  amenities?: Amenity[];
};

export interface UpdateSchedulesRequest {
  schedules: {
    day: DayOfWeek;
    open_time: string;
    close_time: string;
    is_closed: boolean;
  }[];
}

export interface BlockedSlot {
  id: string;
  court_id: string;
  date: string;
  start_time: string;
  end_time: string;
  reason: string | null;
  created_by?: string | null;
  created_at: string;
  court_name?: string;
}
