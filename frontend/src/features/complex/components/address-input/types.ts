export interface Prediction {
  place_id: string;
  description: string;
  structured_formatting: {
    main_text: string;
    secondary_text: string;
  };
}

export interface PlaceDetails {
  address: string;
  city: string;
  province: string;
  formatted_address: string;
  latitude: string;
  longitude: string;
}

export interface AddressSelection {
  address: string;
  city: string;
  province: string;
  formatted_address: string;
  latitude: number;
  longitude: number;
}

export function generateSessionToken() {
  return crypto.randomUUID();
}
