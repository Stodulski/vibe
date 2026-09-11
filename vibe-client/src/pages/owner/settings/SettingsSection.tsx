/**
 * A titled group of settings inside a tab.
 *
 * Merging five sections into three made two of the tabs hold more than one
 * subject, and a long undifferentiated field list is exactly what headings
 * exist to break up. The rule is only worth following if the heading is real:
 * each one names a group the reader could act on separately.
 */
export function SettingsSection({
  title,
  children,
  className,
}: {
  title: string;
  children: React.ReactNode;
  className?: string;
}) {
  return (
    <section className={className}>
      <h3 className="mb-3 text-micro font-medium uppercase tracking-wider text-text-tertiary">{title}</h3>
      {children}
    </section>
  );
}
