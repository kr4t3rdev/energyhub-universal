export type Freshness = "missing" | "fresh" | "stale";

export interface Reading {
  label: string;
  value: number | null;
  unit: string;
  icon: string;
  note: string;
  /** ISO 8601 del momento de la medida, o null si no hay dato. */
  measuredAt: string | null;
}

export interface DeviceSummary {
  id: string;
  name: string;
  kind: string;
  /** Estado del equipo, o null si no hay dato. */
  status: string | null;
  /** Nota de presentación; en la fixture siempre es "Valor de ejemplo". */
  note: string;
}

export interface SiteOverview {
  name: string;
  /** ISO 8601 de la última actualización real, o null si nunca hubo datos. */
  updatedAt: string | null;
  readings: Reading[];
  devices: DeviceSummary[];
}
