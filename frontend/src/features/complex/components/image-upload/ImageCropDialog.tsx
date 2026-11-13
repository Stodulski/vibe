import { useState } from 'react';
import Cropper from 'react-easy-crop';
import { Button } from '@/shared/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/shared/components/ui/dialog';
import { ES_AR } from '@/shared/i18n/es_AR';
import type { CropArea } from '../../lib/cropImage';

const t = ES_AR;

const ZOOM_MIN = 1;
const ZOOM_MAX = 3;

function ZoomSlider({ zoom, onChange }: { zoom: number; onChange: (zoom: number) => void }) {
  return (
    <label className="flex items-center gap-3 text-sm">
      <span className="text-text-secondary">{t.complex.cropZoom}</span>
      <input
        type="range"
        min={ZOOM_MIN}
        max={ZOOM_MAX}
        step={0.01}
        value={zoom}
        onChange={(e) => {
          onChange(Number(e.target.value));
        }}
        className="accent-primary-500 h-6 flex-1"
      />
    </label>
  );
}

/**
 * Lets the owner decide which part of the picked image is kept, at the fixed
 * ratio the image is shown at (16:9 cover, square logo). The crop is only
 * reported, not applied: the caller crops and uploads on `onConfirm`.
 */
export function ImageCropDialog({
  src,
  aspect,
  onCancel,
  onConfirm,
}: {
  src: string;
  aspect: number;
  onCancel: () => void;
  onConfirm: (area: CropArea) => void;
}) {
  const [crop, setCrop] = useState({ x: 0, y: 0 });
  const [zoom, setZoom] = useState(ZOOM_MIN);
  const [area, setArea] = useState<CropArea | null>(null);

  return (
    <Dialog
      open
      onOpenChange={(open) => {
        if (!open) onCancel();
      }}
    >
      <DialogContent showCloseButton={false}>
        <DialogHeader>
          <DialogTitle>{t.complex.cropTitle}</DialogTitle>
          <DialogDescription>{t.complex.cropHint}</DialogDescription>
        </DialogHeader>
        <div className="bg-bg-base relative min-h-64 flex-1 overflow-hidden rounded-lg sm:h-80 sm:flex-none">
          <Cropper
            image={src}
            crop={crop}
            zoom={zoom}
            minZoom={ZOOM_MIN}
            maxZoom={ZOOM_MAX}
            aspect={aspect}
            onCropChange={setCrop}
            onZoomChange={setZoom}
            onCropComplete={(_, pixels) => {
              setArea(pixels);
            }}
          />
        </div>
        <ZoomSlider zoom={zoom} onChange={setZoom} />
        <DialogFooter>
          <Button type="button" variant="outline" onClick={onCancel}>
            {t.complex.cropCancel}
          </Button>
          <Button
            type="button"
            disabled={!area}
            onClick={() => {
              if (area) onConfirm(area);
            }}
          >
            {t.complex.cropSave}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
