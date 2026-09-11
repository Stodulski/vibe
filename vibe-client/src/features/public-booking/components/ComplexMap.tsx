import { useEffect, useRef } from 'react';
import { MapContainer, TileLayer, Marker, Popup } from 'react-leaflet';
import { icon } from 'leaflet';
import { ExternalLink } from 'lucide-react';
import { ES_AR } from '@/shared/i18n/es_AR';
import 'leaflet/dist/leaflet.css';

const t = ES_AR;

const markerIcon = icon({
  iconUrl: 'https://unpkg.com/leaflet@1.9.4/dist/images/marker-icon.png',
  iconRetinaUrl: 'https://unpkg.com/leaflet@1.9.4/dist/images/marker-icon-2x.png',
  shadowUrl: 'https://unpkg.com/leaflet@1.9.4/dist/images/marker-shadow.png',
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
  const mapRef = useRef<L.Map | null>(null);

  useEffect(() => {
    // Invalidate map size after mount to handle container resize.
    const timer = setTimeout(() => {
      mapRef.current?.invalidateSize();
    }, 100);
    return () => {
      clearTimeout(timer);
    };
  }, []);

  const googleMapsUrl = `https://www.google.com/maps/search/?api=1&query=${String(latitude)},${String(longitude)}`;

  return (
    <div className="overflow-hidden rounded-xl border border-border-subtle">
      <MapContainer
        center={[latitude, longitude]}
        zoom={15}
        scrollWheelZoom={false}
        dragging={!('ontouchstart' in window)}
        className="h-[200px] w-full z-0"
        ref={mapRef}
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
