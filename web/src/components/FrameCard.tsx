import { useEffect, useState } from 'preact/hooks';
import { frameUrl } from '../api';
import type { FrameInfo } from '../types';
import './FrameCard.css';

function info(f: FrameInfo): string {
  if (f.solved) return `RA ${(f.ra_deg / 15).toFixed(4)}h · Dec ${f.dec_deg.toFixed(4)}° · ${f.scale_arcsec ? `${f.scale_arcsec.toFixed(2)}″/px · ` : ''}solved in ${f.solve_s.toFixed(1)} s`;
  if (f.solving) return 'Plate solving…';
  if (f.error) return `Solve failed: ${f.error}`;
  return '';
}

/** The latest exposure, with its solve status. Click to enlarge. */
export function FrameCard({ frame }: { frame: FrameInfo }) {
  // Preload each new frame so the picture swaps instead of flashing empty.
  const [shown, setShown] = useState<number | null>(null);
  const [zoom, setZoom] = useState(false);
  useEffect(() => {
    const img = new Image();
    img.onload = () => setShown(frame.id);
    img.src = frameUrl(frame.id);
  }, [frame.id]);
  useEffect(() => {
    if (!zoom) return;
    const onKey = (e: KeyboardEvent) => e.key === 'Escape' && setZoom(false);
    document.addEventListener('keydown', onKey);
    return () => document.removeEventListener('keydown', onKey);
  }, [zoom]);

  const badge = frame.solving ? 'solving…' : frame.solved ? 'solved' : frame.error ? 'no solve' : '';
  return (
    <div class="card">
      <div class="eyebrow mb-10">Last frame</div>
      <div class="frame" title="Click to enlarge" onClick={() => shown !== null && setZoom(true)}>
        {shown === null ? <div class="empty">No frame yet</div> : <img src={frameUrl(shown)} alt="Latest frame" />}
        {badge && <div class="badge">{badge}</div>}
      </div>
      <div class="small muted mt-10">{info(frame) || ' '}</div>
      {zoom && shown !== null && (
        <div class="lightbox" onClick={() => setZoom(false)}>
          <img src={frameUrl(shown)} alt="Frame" />
        </div>
      )}
    </div>
  );
}
