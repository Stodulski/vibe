// ─── Court ───

export type Sport = 'padel' | 'tennis' | 'soccer' | 'basketball';
export type CourtType = 'indoor' | 'outdoor' | 'semi_covered';
export type DayType = 'monday' | 'tuesday' | 'wednesday' | 'thursday' | 'friday' | 'saturday' | 'sunday';
export type DurationMinutes = 60 | 90 | 120;

export interface Court {
  id: string;
  complex_id: string;
  name: string;
  sport: Sport;
  court_type: CourtType;
  is_active: boolean;
  /**
   * What the court is like — floor, walls, lighting. Absent when the owner has
   * never written one; the server omits the key rather than sending an empty
   * string, so there is no "" to tell apart from "not described".
   */
  description?: string;
  created_at: string;
  updated_at: string;
}

export interface CourtPrice {
  id: string;
  court_id: string;
  price: number;
  day_type: DayType;
  time_from: string;
  time_to: string;
  /**
   * The band as minutes from its weekday's own midnight. `to_min` exceeds 1440
   * for a band running into the next day — Thursday 22:00–01:30 is 1320 to
   * 1530 — which is the whole reason they are sent: comparing the two clock
   * strings puts such a band's end before its start, and a match written that
   * way covers nothing at all.
   *
   * Derived by the database (the span_min generated column) and read here. `time_from` and
   * `time_to` remain what an owner edits.
   */
  from_min: number;
  to_min: number;
}

export interface CourtWithPrices extends Court {
  prices: CourtPrice[];
}

export interface CreateCourtRequest {
  name: string;
  sport: Sport;
  court_type: CourtType;
}

export type UpdateCourtRequest = Partial<CreateCourtRequest> & {
  is_active?: boolean;
};

export interface UpdatePricesRequest {
  prices: {
    price: number;
    day_type: DayType;
    time_from: string;
    time_to: string;
  }[];
}
