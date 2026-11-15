import { useState } from 'react';
import Cropper from 'react-easy-crop';
import { SectionFooter } from '@/shared/components/common/SectionFooter';
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '@/shared/components/ui/dialog';
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

/** Title, hint, and a warning when the source will be enlarged. */
function CropHeader({ small }: { small: boolean }) {
  return (
    <DialogHeader>
      <DialogTitle>{t.complex.cropTitle}</DialogTitle>
      <DialogDescription>{t.complex.cropHint}</DialogDescription>
      {small && <p className="text-warning-text text-xs">{t.complex.cropSmallImage}</p>}
    </DialogHeader>
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
  small = false,
  onCancel,
  onConfirm,
}: {
  src: string;
  aspect: number;
  /** The source is smaller than the uploaded size, so it will be enlarged. */
  small?: boolean;
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
        <CropHeader small={small} />
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
        <SectionFooter
          onCancel={onCancel}
          cancelLabel={t.complex.cropCancel}
          submitLabel={t.complex.cropSave}
          submitDisabled={!area}
          onSubmit={() => {
            if (area) onConfirm(area);
          }}
        />
      </DialogContent>
    </Dialog>
  );
}
