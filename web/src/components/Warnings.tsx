export function Warnings({ items }: { items: readonly string[] | null | undefined }) {
  if (!items || items.length === 0) return null;
  return (
    <>
      {items.map((w, i) => (
        <div class="warn" key={i}>
          <span>{w}</span>
        </div>
      ))}
    </>
  );
}
