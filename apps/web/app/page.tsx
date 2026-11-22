import { connection } from "next/server";
import { Suspense } from "react";
import { siteOverviewProvider } from "./site/provider";
import { deviceStatusLabel, formatReading, freshnessLabel, hasDevices } from "./site/presentation";
import OrganizationsList from "./site/organizations-list";
import { AuthButton } from "./site/auth-button";
import { ThemeToggle } from "./site/theme-toggle";

async function SiteMeta({ updatedAt }: { updatedAt: string | null }) {
  await connection();
  return (
    <div className="site-meta">
      <span className="meta-dot" /> Ejemplo estático <span className="meta-divider">/</span>{" "}
      {freshnessLabel(updatedAt, new Date())}
    </div>
  );
}

export default async function Home() {
  const site = await siteOverviewProvider.getSiteOverview();
  const { readings, devices } = site;

  return (
    <div className="app-shell">
      <a className="skip-link" href="#contenido">Saltar al contenido</a>
      <header className="header">
        <a href="#inicio" className="brand">
          <span className="brand-mark" aria-hidden="true">E<span>↗</span></span>
          <span>energyhub<small>UNIVERSAL</small></span>
        </a>
        <nav aria-label="Navegación principal">
          <a className="nav-active" href="#inicio">Vista general</a>
          <a href="#dispositivos">Dispositivos</a>
          <a href="#acerca">Acerca de</a>
        </nav>
        <span className="header-tag"><span className="status-dot" /> MODO DEMO</span>
        <ThemeToggle />
        <AuthButton />
      </header>

      <main id="contenido">
        <section className="intro container" id="inicio">
          <div>
            <p className="eyebrow"><span className="status-dot" /> TU ENERGÍA, EN CONTEXTO</p>
            <h1>Una mirada más clara<br /><em>a tu energía.</em></h1>
            <p className="intro-copy">Un espacio para entender tu instalación, reunir la información importante y ver todo con perspectiva.</p>
          </div>
          <div className="intro-index" aria-hidden="true"><span>01 / 03</span><span>ENERGY<br />OVERVIEW</span></div>
        </section>

        <section className="dashboard container" aria-labelledby="site-title">
          <div className="demo-banner">
            <span className="demo-icon" aria-hidden="true">✳</span>
            <div><strong>Estás explorando una demostración</strong><p>Todos los valores de esta pantalla son datos de ejemplo. No hay equipos conectados.</p></div>
            <span className="demo-label">DATOS DE EJEMPLO</span>
          </div>

          <div className="section-heading">
            <div><p className="eyebrow muted">INSTALACIÓN / EJEMPLO</p><h2 id="site-title">{site.name}</h2><p>Una vista general de cómo se organizará tu información energética.</p></div>
            <Suspense fallback={<div className="site-meta"><span className="meta-dot" /> Ejemplo estático <span className="meta-divider">/</span> Sin datos disponibles</div>}>
              <SiteMeta updatedAt={site.updatedAt} />
            </Suspense>
          </div>

          <div className="metrics">
            {readings.map((reading, index) => (
              <article className={`metric metric-${index + 1}`} key={reading.label}>
                <div className="metric-top"><span>{reading.label}</span><span aria-hidden="true">{reading.icon}</span></div>
                <p className="metric-value">{formatReading(reading.value, reading.unit)}</p>
                <p className="metric-note">{reading.note}</p>
                <span className="metric-number">0{index + 1}</span>
              </article>
            ))}
          </div>

          <div className="lower-grid">
            <section className="devices-panel" id="dispositivos" aria-labelledby="devices-title">
              <div className="panel-header"><div><p className="eyebrow muted">TU INSTALACIÓN</p><h2 id="devices-title">Dispositivos</h2></div><span className="count">{devices.length.toString().padStart(2, "0")}</span></div>
              {hasDevices(devices) ? (
                <ul className="device-list">
                  {devices.map((device) => (
                    <li className="device-item" key={device.id}>
                      <div><span className="device-name">{device.name}</span><span className="device-kind">{device.kind}</span></div>
                      <p className="device-status">{deviceStatusLabel(device)}</p>
                      <p className="device-note">{device.note}</p>
                    </li>
                  ))}
                </ul>
              ) : (
                <div className="empty-state">
                  <div className="empty-symbol" aria-hidden="true"><span>＋</span></div>
                  <div><h3>Aún no hay dispositivos</h3><p>Cuando la conexión esté disponible, aquí podrás consultar los equipos de tu instalación.</p></div>
                </div>
              )}
              <p className="panel-footnote"><span aria-hidden="true">↗</span> La incorporación de equipos desde el móvil llegará en una fase posterior.</p>
            </section>

            <aside className="note-panel" id="acerca" aria-labelledby="note-title">
              <span className="note-spark" aria-hidden="true">✳</span>
              <p className="eyebrow">HECHO PARA DARTE CLARIDAD</p>
              <h2 id="note-title">Cada dato<br /><em>en su lugar.</em></h2>
              <p>EnergyHub Universal está en desarrollo. Esta primera vista es una propuesta visual, no una conexión con equipos reales.</p>
              <span className="note-bottom">ENERGYHUB UNIVERSAL <span>↗</span></span>
            </aside>
          </div>
        </section>

        <section className="organizations-section container" id="organizaciones" aria-labelledby="org-title">
          <OrganizationsList />
        </section>
      </main>
      <footer className="footer container"><span>© EnergyHub Universal</span><span>UNA NUEVA PERSPECTIVA PARA TU ENERGÍA <span aria-hidden="true">✳</span></span><span>PROYECTO EN DESARROLLO</span></footer>
    </div>
  );
}
