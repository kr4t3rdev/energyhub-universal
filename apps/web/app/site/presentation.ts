import type { Freshness } from "./types";

const DEFAULT_STALE_AFTER_MINUTES = 30;

/**
 * Clasifica la frescura de una lectura respecto de `now`.
 * Las fechas inválidas o ausentes son "missing": nunca se infiere un valor.
 * Un timestamp futuro se tolera como "fresh" (desviación de reloj).
 */
export function readingFreshness(
  measuredAt: string | null,
  now: Date,
  staleAfterMinutes: number = DEFAULT_STALE_AFTER_MINUTES,
): Freshness {
  if (measuredAt === null) return "missing";
  const measured = new Date(measuredAt);
  if (Number.isNaN(measured.getTime())) return "missing";
  const ageMs = now.getTime() - measured.getTime();
  if (ageMs < 0) return "fresh";
  return ageMs <= staleAfterMinutes * 60_000 ? "fresh" : "stale";
}

/** Etiqueta en español para la frescura, sin inventar datos. */
export function freshnessLabel(
  measuredAt: string | null,
  now: Date,
  staleAfterMinutes: number = DEFAULT_STALE_AFTER_MINUTES,
): string {
  const freshness = readingFreshness(measuredAt, now, staleAfterMinutes);
  if (freshness === "missing") return "Sin datos disponibles";
  const measured = new Date(measuredAt as string);
  const minutes = Math.max(0, Math.floor((now.getTime() - measured.getTime()) / 60_000));
  const elapsed =
    minutes < 1 ? "ahora" : minutes < 60 ? `hace ${minutes} min` : `hace ${Math.floor(minutes / 60)} h`;
  return freshness === "fresh" ? `Actualizado ${elapsed}` : `Datos desactualizados · ${elapsed}`;
}

export function formatReading(value: number | null, unit: string): string {
  if (value === null || !Number.isFinite(value)) return "—";
  const formatted = new Intl.NumberFormat("es", {
    maximumFractionDigits: 2,
  }).format(value);
  return unit ? `${formatted} ${unit}` : formatted;
}
