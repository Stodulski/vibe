import { useEffect, useState } from 'react';
import type { Map as LeafletMap } from 'leaflet';
import { MapContainer, TileLayer, Marker, Popup } from 'react-leaflet';
import { icon } from 'leaflet';
import { ExternalLink } from 'lucide-react';
import { ES_AR } from '@/shared/i18n/es_AR';
// Leaflet ships these with the package but its CSS expects them next to the
// stylesheet, which no bundler reproduces; importing them as modules gives
// Vite hashed, same-origin URLs it also precaches. They used to be loaded
// from unpkg.com at runtime: a third-party CDN in the render path of the
// storefront, broken offline and under a strict CSP (MAP-03).
import markerIconUrl from '@/assets/leaflet/marker-icon.png';
import markerIcon2xUrl from '@/assets/leaflet/marker-icon-2x.png';
import markerShadowUrl from '@/assets/leaflet/marker-shadow.png';
import 'leaflet/dist/leaflet.css';

const t = ES_AR;

const markerIcon = icon({
  iconUrl: markerIconUrl,
  iconRetinaUrl: markerIcon2xUrl,
  shadowUrl: markerShadowUrl,
  iconSize: [25, 41],
  iconAnchor: [12, 41],
  popupAnchor: [1, -34],
  shadowSize: [41, 41],
});

interface ComplexMapProps {
  latitude: number;
  longitude: number;
  name: string;
  address: string;
}

export function ComplexMap({ latitude, longitude, name, address }: ComplexMapProps) {
  // State, not a ref: react-leaflet builds the Leaflet map in its own effect
  // and only then fills the forwarded ref, so an effect here reading a ref
  // sees `null` on the pass that matters and never runs again. Holding the
  // instance in state re-runs the effects below the moment it arrives.
  const [map, setMap] = useState<LeafletMap | null>(null);

  useEffect(() => {
    if (!map) return;
    // Invalidate map size after mount to handle container resize.
    const timer = setTimeout(() => {
      map.invalidateSize();
    }, 100);
    return () => {
      clearTimeout(timer);
    };
  }, [map]);

  // Leaflet's container is keyboard-focusable and pans with the arrow keys,
  // so it cannot be hidden from assistive tech — but it was reaching it as
  // an unnamed box of tiles (A11Y-11). `application` is what it is: a widget
  // that consumes the arrow keys itself. Anyone who would rather not drive a
  // map still has the address as text above it and the Google Maps link
  // below. Set here rather than as JSX props because react-leaflet's
  // MapContainer only forwards className, id and style to the element.
  useEffect(() => {
    const container = map?.getContainer();
    if (!container) return;
    container.setAttribute('role', 'application');
    container.setAttribute('aria-label', `${t.complex.mapOf} ${name}, ${address}`);
  }, [map, name, address]);

  const googleMapsUrl = `https://www.google.com/maps/search/?api=1&query=${String(latitude)},${String(longitude)}`;

  return (
    <div className="overflow-hidden rounded-xl border border-border-subtle">
      <MapContainer
        center={[latitude, longitude]}
        zoom={15}
        scrollWheelZoom={false}
        dragging={!('ontouchstart' in window)}
        className="h-[200px] w-full z-0"
        ref={setMap}
      >
        <TileLayer
          attribution='&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a>'
          url="https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png"
        />
        <Marker position={[latitude, longitude]} icon={markerIcon}>
          <Popup>
            <span className="text-xs font-medium">{name}</span>
            <br />
            <span className="text-xs text-text-secondary">{address}</span>
          </Popup>
        </Marker>
      </MapContainer>
      <a
        href={googleMapsUrl}
        target="_blank"
        rel="noopener noreferrer"
        className="flex items-center justify-center gap-2 bg-bg-subtle px-4 py-2.5 text-xs font-medium text-primary-400 transition-colors hover:bg-bg-base hover:text-primary-300"
      >
        <ExternalLink className="size-3.5" />
        {t.complex.viewOnGoogleMaps}
      </a>
    </div>
  );
}
