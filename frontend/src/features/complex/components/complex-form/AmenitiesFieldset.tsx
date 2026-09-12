import { Controller } from 'react-hook-form';
import { Checkbox } from '@/shared/components/ui/checkbox';
import { ES_AR } from '@/shared/i18n/es_AR';
import { cn } from '@/shared/lib/utils';
import { AMENITIES } from '@/shared/lib/amenities';
import type { Control } from 'react-hook-form';
import type { Amenity } from '@/shared/types/api.types';
import type { CreateComplexDto } from '../../schemas/complex.schema';

const t = ES_AR;

/**
 * The services a venue offers, as a group of checkboxes inside the complex
 * form.
 *
 * Checkboxes rather than a multi-select: there are twelve, they all fit, and
 * the question being answered is "which of these do I have" — faster to answer
 * from a visible list than from one hidden behind an interaction.
 *
 * A field of the form, not a form of its own. It used to have its own save
 * button, which put a second submit on a screen that asks one question; now it
 * goes with everything else when "Guardar cambios" is pressed.
 */
export function AmenitiesFieldset({ control }: { control: Control<CreateComplexDto> }) {
  return (
    <Controller
      control={control}
      name="amenities"
      render={({ field }) => {
        const selected: Amenity[] = field.value;
        const toggle = (value: Amenity, checked: boolean) => {
          const next = checked ? [...selected, value] : selected.filter((a) => a !== value);
          // Stored in the canonical order rather than the order they were
          // ticked, so two venues offering the same things store the same
          // array and their public pages list them the same way.
          field.onChange(AMENITIES.filter((a) => next.includes(a.value)).map((a) => a.value));
        };

        return (
          <fieldset className="@container space-y-3">
            {/* Same size and colour as the labels above it — `Label`'s own
                `text-text-secondary`, at the `text-xs` this form overrides to.
                A legend a shade brighter than every label around it reads as a
                heading for the rest of the form, not as this group's name. */}
            <legend className="text-xs font-medium leading-none text-text-secondary">
              {t.complex.amenitiesSection}
            </legend>
            {/* Two columns once the container can seat them, one below.
                Stacked checkboxes give a single downward path, which is the
                safer default — but twelve of them is 530px of scroll, and the
                save button ends up somewhere nobody has reached.

                The threshold is the container's, not the viewport's, and it is
                where the longest label stops fitting: at a 252px container
                (320px phone) two columns leave 62px for the text and
                "Grabación de partidos" needs about 147px, so it would truncate
                to eight characters. At 576px each column has 224px and every
                label fits whole. */}
            <div className="grid grid-cols-1 gap-x-4 gap-y-1 @sm:grid-cols-2">
              {AMENITIES.map((amenity) => (
                <AmenityCheckbox
                  key={amenity.value}
                  amenity={amenity}
                  checked={selected.includes(amenity.value)}
                  onToggle={toggle}
                />
              ))}
            </div>
          </fieldset>
        );
      }}
    />
  );
}

/**
 * One amenity, as a full-row target.
 *
 * The whole row is the label, not just the words: a 16px checkbox is well
 * under the minimum target size, and the row gives it 44px of height and the
 * full column width to be hit with.
 */
function AmenityCheckbox({
  amenity,
  checked,
  onToggle,
}: {
  amenity: (typeof AMENITIES)[number];
  checked: boolean;
  onToggle: (value: Amenity, checked: boolean) => void;
}) {
  return (
    <label className="flex h-11 cursor-pointer items-center gap-3 rounded-lg px-2 transition-colors hover:bg-bg-elevated/60">
      <Checkbox
        checked={checked}
        onCheckedChange={(value) => {
          onToggle(amenity.value, value === true);
        }}
      />
      <amenity.icon
        className={cn('size-4 shrink-0', checked ? 'text-primary-400' : 'text-text-tertiary')}
        aria-hidden="true"
      />
      <span className={cn('truncate text-sm', checked ? 'text-text-primary' : 'text-text-secondary')}>
        {amenity.label}
      </span>
    </label>
  );
}
