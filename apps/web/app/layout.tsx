import type { Metadata } from "next";
import "./globals.css";

export const metadata: Metadata = {
  title: "EnergyHub Universal",
  description: "Una perspectiva más clara de tu energía. Demostración con datos de ejemplo.",
};

export default function RootLayout({ children }: Readonly<{ children: React.ReactNode }>) {
  return <html lang="es"><body>{children}</body></html>;
}
