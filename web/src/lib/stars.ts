/** A tileable CSS `url(...)` of faint stars, for the page backdrop. */
export function starfield(color: string): string {
  let seed = 7;
  const rnd = () => (seed = (seed * 16807) % 2147483647) / 2147483647;
  let dots = '';
  for (let i = 0; i < 70; i++) {
    const cx = (rnd() * 700).toFixed(0);
    const cy = (rnd() * 700).toFixed(0);
    const r = (0.5 + rnd() * 1.1).toFixed(1);
    const o = (0.08 + rnd() * 0.3).toFixed(2);
    dots += `<circle cx="${cx}" cy="${cy}" r="${r}" fill="${color}" opacity="${o}"/>`;
  }
  const svg = `<svg xmlns="http://www.w3.org/2000/svg" width="700" height="700">${dots}</svg>`;
  return `url("data:image/svg+xml,${encodeURIComponent(svg)}")`;
}
