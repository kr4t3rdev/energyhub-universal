"use client";

/**
 * Botón para cambiar entre modo claro y oscuro.
 * El tema vive en `document.documentElement.dataset.theme` y se guarda
 * en localStorage (clave `energyhub_theme`); el icono lo elige el CSS
 * según el tema activo, así no hay parpadeo ni desajuste de hidratación.
 */
export function ThemeToggle() {
  const toggleTheme = () => {
    const root = document.documentElement;
    const next = root.dataset.theme === "dark" ? "light" : "dark";
    root.dataset.theme = next;
    try {
      localStorage.setItem("energyhub_theme", next);
    } catch {
      // Sin almacenamiento: el tema solo aplica a esta visita.
    }
  };

  return (
    <button
      type="button"
      className="theme-toggle"
      onClick={toggleTheme}
      aria-label="Cambiar entre modo claro y oscuro"
      title="Cambiar tema"
    >
      <span className="theme-icon-moon" aria-hidden="true">☾</span>
      <span className="theme-icon-sun" aria-hidden="true">☀</span>
    </button>
  );
}
