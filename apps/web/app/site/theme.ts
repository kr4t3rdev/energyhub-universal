/** Tema de la web: claro u oscuro. */
export type ThemeName = "light" | "dark";

export const THEME_STORAGE_KEY = "energyhub_theme";

/**
 * Resuelve el tema inicial: el guardado en localStorage manda;
 * sin valor guardado (o inválido) se sigue la preferencia del sistema.
 */
export function resolveTheme(stored: string | null, prefersDark: boolean): ThemeName {
  if (stored === "dark" || stored === "light") {
    return stored;
  }
  return prefersDark ? "dark" : "light";
}
