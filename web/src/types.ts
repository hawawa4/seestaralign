// The JSON the Go server sends. Keep in sync with internal/session (State and
// friends), internal/polar (Error), internal/models and internal/alpaca.

export type Phase = 'disconnected' | 'connecting' | 'ready' | 'measuring' | 'adjusting';
export type StartMode = 'altaz' | 'radec';
export type Direction = 'east' | 'west';

export interface Settings {
  host: string;
  port: number; // 0 = discover, then 32323
  camera_num: number;
  telescope_num: number;
  model: string; // '' = auto
  lat: number | null;
  lon: number | null;

  exposure_s: number;
  gain: number | null;
  move_deg: number;
  move_rate: number;
  direction: Direction;
  settle_s: number;
  search_radius_deg: number;

  start_enabled: boolean;
  start_mode: StartMode;
  start_alt_deg: number;
  start_az_deg: number;
  start_ra_hours: number;
  start_dec_deg: number;

  refraction: boolean;
  pressure_hpa: number;
  temp_c: number;

  astap_path: string;
  astap_database: string;
  astap_database_dir: string;
  sync_clock: boolean;
}

export interface Device {
  type: string;
  number: number;
  name: string;
  unique_id: string;
}

export interface Model {
  key: string;
  name: string;
  focal_mm: number;
  pixel_um: number;
  width: number;
  height: number;
}

export interface RigInfo {
  base_url: string;
  camera: Device;
  telescope: Device;
  sensor_w: number;
  sensor_h: number;
  pixel_um: number;
  lat: number | null;
  lon: number | null;
  alignment_mode: number; // 0 alt-az, 1 polar, 2 german polar, -1 unknown
  can_set_tracking: boolean;
  clock_offset_s: number | null;
  detected_model: Model | null;
}

export interface Site {
  lat: number;
  lon: number;
  source: 'mount' | 'override' | 'none';
}

export interface Point {
  n: number;
  ra_deg: number;
  dec_deg: number;
  alt_deg: number;
  az_deg: number;
  time: string;
}

export interface PolarError {
  alt_deg: number; // + : axis too high
  az_deg: number; // + : axis east of the pole
  total_deg: number;
  separation_deg: number;
  northern: boolean;
  alt_direction: 'raise' | 'lower';
  az_direction: 'east' | 'west';
  az_hand: 'left' | 'right'; // facing the pole
}

export interface HistoryEntry {
  t: string;
  total: number;
  alt: number;
  az: number;
}

export interface AdjustInfo {
  frames: number;
  conditioning: number;
  alpha_deg: number;
  beta_deg: number;
  warnings: string[] | null;
}

export interface FrameInfo {
  id: number;
  time: string;
  solved: boolean;
  solving: boolean;
  ra_deg: number;
  dec_deg: number;
  scale_arcsec: number;
  solve_s: number;
  error?: string;
}

export type LogLevel = 'info' | 'warn' | 'error';
export interface LogEntry {
  t: string;
  level: LogLevel;
  msg: string;
}

export interface AppState {
  phase: Phase;
  step: string;
  slewing: boolean;
  settings: Settings;
  rig: RigInfo | null;
  site: Site | null;
  model: Model | null;
  model_source: string;
  models: Model[];
  points: Point[] | null;
  initial: PolarError | null;
  error: PolarError | null;
  history: HistoryEntry[] | null;
  adjust: AdjustInfo | null;
  frame: FrameInfo | null;
  warnings: string[] | null;
  log: LogEntry[] | null;
  astap: { path: string; databases: string[] | null };
}

export interface FoundDevice {
  ip: string;
  port: number;
}
