import type { Metadata } from "next";
import Script from "next/script";
import "./globals.css";

export const metadata: Metadata = {
  title: "EnergyHub Universal",
  description: "Una perspectiva más clara de tu energía. Demostración con datos de ejemplo.",
};

/**
 * Aplica el tema antes de pintar la página para que no haya flash:
 * el guardado en localStorage manda; si no hay, se sigue el sistema.
 */
const themeInitScript = `(function(){try{var stored=localStorage.getItem('energyhub_theme');var theme=(stored==='dark'||stored==='light')?stored:(window.matchMedia('(prefers-color-scheme: dark)').matches?'dark':'light');document.documentElement.dataset.theme=theme;}catch(e){}})();`;

export default function RootLayout({ children }: Readonly<{ children: React.ReactNode }>) {
  return (
    <html lang="es" suppressHydrationWarning>
      <Script id="theme-init" strategy="beforeInteractive">
        {themeInitScript}
      </Script>
      <body>{children}</body>
    </html>
  );
}
