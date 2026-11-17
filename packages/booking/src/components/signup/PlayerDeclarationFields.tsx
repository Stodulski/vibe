import { get, type FieldErrors, type FieldValues, type Path, type UseFormRegister } from 'react-hook-form';
import { FieldRequirement, FormField } from '@vibe/ui';
import { BOOKING_MESSAGES } from '../../messages';
import { declarationWords, levelLabel } from '../../lib/traitLabels';
import { SELECT_CLASS } from '../selectStyles';

const d = BOOKING_MESSAGES.declaration;

/** The levels a person can declare: 1ra (1) to 9na (9). */
const LEVELS = [1, 2, 3, 4, 5, 6, 7, 8, 9] as const;

interface GenderChoiceProps<T extends FieldValues> {
  register: UseFormRegister<T>;
  name: Path<T>;
  idPrefix: string;
  error: string | undefined;
  legend: string;
  options: readonly { value: string; label: string }[];
}

/**
 * The gender as a segmented choice: a radio for each, drawn as a button, so a
 * tap and the keyboard both choose it.
 */
function GenderChoice<T extends FieldValues>({ register, name, idPrefix, error, legend, options }: GenderChoiceProps<T>) {
  const errorId = `${idPrefix}gender-error`;
  return (
    <fieldset className="flex flex-col gap-2" aria-describedby={error ? errorId : undefined}>
      <legend className="text-text-primary mb-2 text-sm font-medium">
        {legend}
        <FieldRequirement required />
      </legend>
      <div className="grid grid-cols-2 gap-2">
        {options.map((option) => (
          <label key={option.value} className="relative cursor-pointer">
            <input type="radio" value={option.value} className="peer sr-only" {...register(name)} />
            <span className="press-scale text-text-primary border-border-interactive bg-bg-subtle peer-checked:border-primary-500 peer-checked:bg-primary-500/10 peer-checked:text-primary-400 peer-focus-visible:ring-primary-500/30 peer-focus-visible:ring-2 flex h-12 items-center justify-center rounded-xl border text-base font-semibold transition-colors duration-200">
              {option.label}
            </span>
          </label>
        ))}
      </div>
      {error && (
        <p id={errorId} role="alert" className="text-error-text flex items-center gap-1.5 text-xs">
          {error}
        </p>
      )}
    </fieldset>
  );
}

interface PlayerDeclarationFieldsProps<T extends FieldValues> {
  register: UseFormRegister<T>;
  errors: FieldErrors<T>;
  /** Where the person's fields live in the form: `''` for the payer, `'partner.'` for the partner. */
  namePrefix?: string;
  /** Keeps the element ids of two people on one page apart. */
  idPrefix: string;
  /** The sport decides the words of the gender: "Caballero / Dama" or "Masculino / Femenino". */
  sport: string;
  /** The category or event has a modality, so the person says their gender. */
  asksGender: boolean;
  /** The category has a level or a Suma, or the event a range, so the person says their level. */
  asksLevel: boolean;
}

/**
 * One person's declaration: their gender (a segmented choice) when the category
 * or event has a modality, and their level, the 1ra to 9na the rules are checked
 * against, when it has a level, a Suma or a range.
 */
export function PlayerDeclarationFields<T extends FieldValues>({
  register,
  errors,
  namePrefix = '',
  idPrefix,
  sport,
  asksGender,
  asksLevel,
}: PlayerDeclarationFieldsProps<T>) {
  const path = (field: string) => `${namePrefix}${field}` as Path<T>;
  const errorOf = (field: string) => (get(errors, path(field)) as { message?: string } | undefined)?.message;
  const levelError = errorOf('level');
  const words = declarationWords(sport);
  const genders = [
    { value: 'men', label: words.gender.men },
    { value: 'women', label: words.gender.women },
  ];

  return (
    <div className={asksGender && asksLevel ? 'grid grid-cols-1 gap-4 sm:grid-cols-2' : 'grid grid-cols-1 gap-4'}>
      {asksGender && (
        <GenderChoice
          register={register}
          name={path('gender')}
          idPrefix={idPrefix}
          error={errorOf('gender')}
          legend={words.genderLabel}
          options={genders}
        />
      )}

      {asksLevel && (
        <FormField
          label={
            <>
              {d.levelLabel}
              <FieldRequirement required />
            </>
          }
          htmlFor={`${idPrefix}level`}
          error={levelError}
        >
          <select
            id={`${idPrefix}level`}
            className={SELECT_CLASS}
            aria-invalid={!!levelError}
            {...register(path('level'))}
          >
            <option value="">{d.levelPlaceholder}</option>
            {LEVELS.map((level) => (
              <option key={level} value={String(level)}>
                {levelLabel(level)}
              </option>
            ))}
          </select>
        </FormField>
      )}
    </div>
  );
}
