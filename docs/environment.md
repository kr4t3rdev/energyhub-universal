# Bootstrap 00 — environment and verification

Date: 2026-11-15. Repository: `/Volumes/Server/Proyectos/energyhub-universal` (external volume). `df -h`: approximately 559 GiB free at start. Directory creation and writes succeeded; no existing directory was overwritten.

Tools observed: Git 2.51.0; Node 22.23.4; pnpm 11.0.8; Go 1.27.1; Xcode 27.0 (27A266a); Swift 6.4; Java 21.0.12; Docker CLI 29.8.0 and Compose 5.5.1. No Android SDK was found at the standard home path or configured in ANDROID_HOME / ANDROID_SDK_ROOT; no Gradle executable found. No available iOS Simulator runtime was listed. XcodeGen was installed and generated the committed Xcode project.

## Verification

- Backend RED: `go test ./internal/platform/httpapi` failed with undefined `NewRouter`, before implementation.
- Backend GREEN: `go test ./...` passed; `go build ./...` passed. Chi 5.5.0 and pgx 5.13.0 downloaded and recorded in go.mod/go.sum. pgx is reserved for database work; no database access exists yet.
- iOS: `xcodebuild -project EnergyHub.xcodeproj -scheme EnergyHub -sdk iphonesimulator -configuration Debug -derivedDataPath <repo>/.local/DerivedData CODE_SIGNING_ALLOWED=NO build` passed. No runnable simulator runtime; smoke test not executed. Host Xcode settings unexpectedly directed build products and intermediates to `/Volumes/Server/XCode`; the build log also reported removing a stale Cashflow intermediate file there. Future builds must set `SYMROOT` and `OBJROOT` inside the repository explicitly.
- Web: `pnpm create next-app@latest ...` generated files but installation failed with `ERR_PNPM_META_FETCH_FAIL` for @tailwindcss/node following network timeouts. No pnpm lockfile, lint, smoke or build verified. Retry `cd apps/web && pnpm install`, then `pnpm lint && pnpm build`; commit the resulting lockfile after verification.
- Android: project scaffold only; no SDK, Gradle wrapper, dependency lock or build/test verified. Install/configure Android SDK and an appropriate Gradle 8.11.1 locally before producing a wrapper, locking dependencies and running `:app:testDebugUnitTest :app:assembleDebug`. The current unit test is only a placeholder, not a UI smoke test.
- Docker: `docker info` failed because the Docker daemon socket was unavailable. Compose is untested. Configure a local `POSTGRES_PASSWORD` in the shell, start Docker, then `docker compose -f infrastructure/compose.yaml up -d` and inspect health; do not commit credentials.
- CI is configured but has not run. Web CI requires a successful installation and lockfile. No remote configured and no initial commit created.

Directories under `device-catalog`, `fixtures`, `simulators` and `sdks` are placeholders only. Hardware protocol research, actual simulators, device compatibility, migration integration, contract tests and native smoke execution remain pending. No auth, billing, or connectors are implemented.

## Dependency installation — 2026-11-15 (later session)

Workspace: `/Volumes/Server/Proyectos/energyhub-universal` (branch `main`, no initial commit yet). All local tooling lives under `.local/` (gitignored). `.local/env.sh` exports `ANDROID_HOME`, `ANDROID_SDK_ROOT`, `JAVA_HOME` (21), `GRADLE_USER_HOME`, `GOPATH`, `GOMODCACHE` and `GOCACHE`.

### Installed and verified

- Backend: `go mod download && go mod verify` passed (chi 5.5.0, pgx 5.13.0 already in `go.sum`); `go build ./...` and `go test ./...` passed. `go run ./cmd/api` answered `{"status":"ok"}` on `/health` (process liveness only, no database access yet).
- Web: `pnpm install` succeeded (pnpm 11.0.8, Node 22.23.4), resolving 426 packages and generating `apps/web/pnpm-lock.yaml`; `pnpm lint` and `pnpm build` (Next.js 16.4.6) passed; `pnpm start` served HTTP 200 on `/`.
- iOS: no external packages (no `Package.swift`, no `Package.resolved`, no `Podfile`); `xcodebuild -resolvePackageDependencies` resolved zero sources and `xcodebuild build` succeeded with `SYMROOT`/`OBJROOT` pinned inside `.local/DerivedData`.
- Android: `gradle :app:testDebugUnitTest :app:assembleDebug` passed with Gradle 8.11.1 on JDK 21 (33m48s on this host; the Kotlin/Compose compile dominates). `SmokeTest` ran 1 test with 0 failures and 0 errors, and `app/build/outputs/apk/debug/app-debug.apk` (8.8 MB) was produced with `package=com.energyhub.universal`, `minSdkVersion=26`, `targetSdkVersion=35`, `compileSdkVersion=35`. The unit test is still only a placeholder, not a UI smoke test, and no emulator/device run was performed.
- Docker: `postgres:17.6` image pulled (digest `sha256:00bc8661…`, 666 MB on disk). The stack is up and healthy (`infrastructure-postgres-1`, `127.0.0.1:5434->5432/tcp`); the `energyhub` database and role connect successfully.

### Toolchain installed locally

- Android SDK in `.local/android-sdk`: command-line tools 13114758, `platform-tools` 37.0.1, `platforms;android-35`, `build-tools;35.0.0`; licenses accepted.
- Gradle 8.11.1 in `.local/tools/gradle-8.11.1` (version used by CI). `apps/android/local.properties` points `sdk.dir` at the local SDK.

### Network notes

`registry.npmjs.org` and `dl.google.com` are reachable but slow (13 s per npm metadata request, roughly 600 KB/s). A plain `pnpm install` timed out after 15 minutes; the successful run used `npm_config_fetch_timeout=600000`, `npm_config_fetch_retries=10`, `npm_config_network_concurrency=8` and `--prefer-offline` (2m19s).

### Blockers and deviations

- Host PostgreSQL 17 (Homebrew) already listens on `127.0.0.1:5432` and `127.0.0.1:5433`, so the compose port could not bind as declared. `infrastructure/compose.yaml` was left untouched; `.local/compose.override.yaml` (gitignored) republishes the container as `127.0.0.1:5434:5432` using the `!override` tag. Run the stack with `docker compose -f infrastructure/compose.yaml -f .local/compose.override.yaml up -d`.
- The generated password is stored in `.local/postgres-password` (mode 600, gitignored). Export it as `POSTGRES_PASSWORD` before any compose command; do not commit it.
- `apps/android` still has no Gradle wrapper committed. CI invokes `gradle` directly with version 8.11.1, so a wrapper is optional; generating it would add a binary `gradle-wrapper.jar` to the repository.
- No Notion MCP server is configured in this harness, so the project plan required by `AGENTS.md` could not be updated; this file is the only local evidence record.
- CI has still not run: no remote is configured and no initial commit exists. The new `apps/web/pnpm-lock.yaml` is still untracked and must be committed before `pnpm install --frozen-lockfile` can pass in CI.
- No Android SDK/Gradle was found on the host before this session; hardware compatibility remains unverified and no device connector, authentication or billing was introduced.

## Database moved to the local Homebrew PostgreSQL — 2026-11-15 (later session)

The Docker daemon was consuming too many resources on this Mac, so the project now uses the Homebrew PostgreSQL 17 instance instead of the `postgres:17.6` container.

### Local database

- Instance: PostgreSQL 17.11 using the Homebrew binary, listening on `127.0.0.1:5433` with data directory `~/pgdata-tcp-test`. It is managed by the launchd agent `~/Library/LaunchAgents/dev.tcp.shadow-postgres-5433.plist` (`RunAtLoad` + `KeepAlive`), so it starts with the user session and restarts if it dies. The cluster is shared with other local projects (`tcp_*`, `financemdm_*` databases).
- Host port map (all three are PostgreSQL 17): 5432 → `/Library/PostgreSQL/17/data` (postgres.org install, runs as user `postgres`); 5433 → `~/pgdata-tcp-test` (the one this project uses); 55439 → a temporary cluster under `/tmp/tcp-ledger-pg.*`.
- The Homebrew service `sh.brew.postgresql@17` is registered but failing: its data directory `/opt/homebrew/var/postgresql@17` does not exist, so it never initialized. It is redundant with the working launchd agent and could be unloaded.
- Provisioned with the same contract as `infrastructure/compose.yaml`: role `energyhub` (`LOGIN`, `CREATEDB`, not superuser — least privilege), database `energyhub` owned by that role, UTF-8. `REVOKE ALL ON DATABASE energyhub FROM PUBLIC` plus `GRANT CONNECT, TEMPORARY TO energyhub`.
- The password is the one already generated for the Docker stack, stored in `.local/postgres-password` (mode 600). The role was rotated to it, so credentials are interchangeable between local and Docker.
- Verified end to end over TCP: `psql -h 127.0.0.1 -p 5433 -U energyhub -d energyhub` authenticates with the password and can create and drop tables.

### Configuration

- `.env.local` (repo root, gitignored via `.gitignore:5`, mode 600) holds `DATABASE_URL` and the `POSTGRES_*` variables pointing at `127.0.0.1:5433`. Do not commit it.
- `.local/env.sh` now sources `.env.local` and exports `DATABASE_URL`, so `source .local/env.sh` gives the backend its connection settings. It prints the URL with the password visible in the terminal — keep that output off shared screens and logs.
- The backend still has no database code (`backend/cmd/api/main.go`, `backend/internal/platform/httpapi/router.go` only) and `backend/migrations/` is an empty placeholder. No connection logic was added: Bootstrap 00 keeps health as process liveness and pgx remains reserved for later database work.

### Docker fallback

`postgres:17.6` remains pulled and the compose file is unchanged. To go back to the container: `POSTGRES_PASSWORD=$(cat .local/postgres-password) docker compose -f infrastructure/compose.yaml -f .local/compose.override.yaml up -d` (it publishes `127.0.0.1:5434` because 5432 and 5433 are taken). The container was left stopped; Docker Desktop was shut down during this session.

## Web installation follow-up — 2026-11-15

`pnpm list --depth 0` confirms the project dependencies are installed and `apps/web/pnpm-lock.yaml` exists. The initial retry had registry timeouts; its final process result is unavailable, but the installed state was verified independently. Resolved direct versions: Next 16.4.6, React and React DOM 19.3.0, Tailwind and its Turbopack integration 4.3.3, TypeScript 5.9.3, ESLint 9.39.5, eslint-config-next 16.4.6, @types/node 20.19.43 and React/React DOM types 19.3.0. `pnpm lint` and `pnpm build` both passed; the build also ran TypeScript checks and generated the static routes. Browser smoke testing remains pending. Notion update was attempted but its connector was unavailable and the browser fallback timed out; this entry should be copied to the existing Bootstrap 00 tracking section when access is restored.

## Dependency review — 2026-11-15

- Backend: Go 1.27.1; module resolution succeeded (`go list -m all`); `go test ./...` and `go build ./...` passed.
- iOS: Xcode 27.0 and XcodeGen 2.46.0 are available; the Xcode project exists. `xcrun simctl list runtimes` listed no runtimes, so simulator execution remains blocked. The iOS app has no external package dependencies configured.
- Android: the project-local Android SDK includes platform 35 and build tools 35.0.0; project-local Gradle 8.11.1 and Java 21 are available. Running `:app:testDebugUnitTest :app:assembleDebug` with Gradle offline succeeded; all 41 tasks were up-to-date, so previous cached outputs were reused, including the existing unit-test result. This does not establish emulator or device execution. Gradle wrapper and dependency locking remain absent.
- Docker CLI and Compose are installed, but the Docker daemon is unavailable, so PostgreSQL could not be checked.
- The Notion connector remained unavailable during this review. Update the existing project plan when access is restored.

## Gradle — dependencia de Android

Gradle **8.11.1** es necesario para compilar y probar Android, junto con JDK 21 y Android SDK 35. En este Mac ya está instalado localmente en `.local/tools/gradle-8.11.1`; esa carpeta está excluida de Git y debe instalarse de nuevo al preparar otra máquina. La distribución oficial está disponible en https://gradle.org/releases/; instálala dentro de `.local/tools/` o configura Gradle 8.11.1 por otro medio autorizado. No se requiere instalación global.

Desde la raíz del repositorio, después de configurar el SDK y el JDK, ejecutar:

```sh
GRADLE_USER_HOME="$PWD/.local/gradle-home" \
  .local/tools/gradle-8.11.1/bin/gradle -p apps/android \
  :app:testDebugUnitTest :app:assembleDebug
```

`apps/android/local.properties` debe apuntar al SDK de cada máquina y permanece fuera de Git. Todavía no se ha agregado el Gradle Wrapper ni un bloqueo de dependencias de Gradle. CI instala Gradle 8.11.1 mediante `gradle/actions/setup-gradle`.

## Verificación PostgreSQL local — 2026-11-15

La configuración actual usa PostgreSQL de Homebrew en `127.0.0.1:5433`, no el contenedor como ruta principal. `pg_isready` respondió y una consulta de solo lectura mediante la configuración local conectó a la base `energyhub`: PostgreSQL 17.11. Se mantiene la configuración de Docker como alternativa; no se modificó el servicio compartido ni sus credenciales. Las notas anteriores que indican que Docker estaba detenido describen su estado en el momento de esas comprobaciones, no el estado de la base local.

## Validación visual de Bootstrap 00 — 2026-11-15

La evidencia reproducible de esta ejecución está en `docs/evidence/validacion-visual-bootstrap-2026-11-15/`. Se conservaron los archivos no rastreados existentes y no se modificó PostgreSQL, otro proyecto ni el código de producto.

### Preparación

- Volumen del proyecto: `/Volumes/Server`, aproximadamente `552 GiB` libres al comenzar la ejecución de validación. Las herramientas y evidencias de trabajo permanecen bajo el proyecto, principalmente en `.local/` y `docs/evidence/`.
- Versiones observadas: Node `22.23.4`, pnpm `11.0.8`, Xcode `27.0 (27A266a)`, Swift `6.4`, JDK `21.0.12`, Gradle local `8.11.1`, Android SDK Platform `35`, Build Tools `35.0.0` y adb `37.0.1`.
- iOS: `xcrun simctl list runtimes` no lista runtimes instalados. Las entradas de dispositivos iOS aparecen como no disponibles. El intento específico `xcodebuild -downloadPlatform iOS -buildVersion 26.1 -architectureVariant arm64` encontró un runtime oficial de `8.32 GB`, pero solo descargó aproximadamente `47 MB` antes del timeout; no se instaló ningún runtime. iOS 26.0 no estaba disponible para descarga.
- Android: adb no encontró dispositivos. El AVD disponible en el perfil del usuario está corrupto; no se modificó. La instalación de `emulator` y `system-images;android-35;google_apis;arm64-v8a` falló al obtener los manifiestos remotos por excepciones de red, así que no se creó un AVD dentro del proyecto.
- CodeGraph no está instalado en el entorno (`gentle-ai` no existe); la inspección estructural se limitó a los archivos relevantes ya indicados por el plan.

### Web

- `pnpm install --frozen-lockfile`: correcto y sin cambios pendientes de instalación.
- `pnpm lint`: correcto.
- `pnpm build`: correcto con Next.js `16.4.6`.
- `pnpm start` respondió `200 OK`. La inspección confirmó que el contenido actual sigue siendo la plantilla inicial de Next.js, no una pantalla de producto.
- La inspección DOM observó cero errores de consola y ningún desbordamiento horizontal en el viewport disponible `1000×700`. La preferencia oscura y el contorno nativo de foco fueron observados; la secuencia completa de teclado no quedó validada.
- No se pudieron producir capturas ni probar los tres tamaños solicitados porque el navegador integrado perdió la conexión y la sesión no tenía una herramienta local de captura. Detalle en `web/browser-dom-evidence.md`.
- Reintento: el navegador volvió a cargar la aplicación y Lighthouse integrado terminó con `100/100` en accesibilidad, buenas prácticas y SEO; `browser.screenshot` siguió bloqueado por no haber una pestaña visible. Los informes están en `web/lighthouse-retry.json` y `web/lighthouse-retry.html`.
- Segundo intento de captura: Brave headless no produjo PNG por `CVDisplayLinkCreateWithCGDisplay failed`; Brave GUI no permitió consultar la ventana por la restricción de accesibilidad de macOS (`-1719`). No se guardaron capturas falsas. El servidor temporal se cerró y el puerto 3000 quedó detenido.

### iOS

- `xcodebuild ... -sdk iphonesimulator ... -derivedDataPath "$PWD/.local/DerivedData/ios-validation" SYMROOT="$PWD/.local/DerivedData/ios-build-products" OBJROOT="$PWD/.local/DerivedData/ios-intermediates" CODE_SIGNING_ALLOWED=NO build`: correcto. El producto quedó en `.local/DerivedData/ios-build-products/Debug-iphonesimulator/EnergyHub.app`; la salida no usa las rutas compartidas observadas en los settings por defecto (`/Volumes/Server/XCode`).
- `xcodebuild ... -destination "generic/platform=iOS Simulator" ... test`: no ejecutable; Xcode exige un dispositivo concreto para `EnergyHubTests` y no existe runtime para crear uno.
- Reintento de runtime: iOS 26.1 fue localizable, pero su descarga de 8.32 GB fue cancelada por timeout tras aproximadamente 47 MB. Detalle en `ios/runtime-retry-summary.md`.
- No se pudo instalar, abrir ni capturar la app. El texto disponible en el código es `EnergyHub Universal`; esto no equivale a una revisión visual ejecutada.

### Android

- `GRADLE_USER_HOME="$PWD/.local/gradle-home" .local/tools/gradle-8.11.1/bin/gradle -p apps/android :app:testDebugUnitTest :app:assembleDebug --console=plain`: correcto; las 41 tareas quedaron `UP-TO-DATE`, incluyendo `testDebugUnitTest` y `assembleDebug`.
- La prueba unitaria existente ejecutó una sola comprobación y no es una prueba de interfaz.
- El APK se construyó en `apps/android/app/build/outputs/apk/debug/app-debug.apk`, con paquete `com.energyhub.universal`, `minSdkVersion=26`, `targetSdkVersion=35`.
- Sin emulador ni dispositivo, no se pudo instalar, abrir, revisar temas/barras/texto ampliado ni capturar la app. No se declara validación visual Android.
- Reintento de herramientas: la descarga oficial del Emulator macOS arm64 se interrumpió por timeout; no se instaló un paquete parcial. Detalle en `android/runtime-retry-summary.md`.

### Notion y cierre

Se intentó acceder al enlace de Notion indicado; la recuperación solo devolvió la página de inicio de Notion y no existe en esta sesión una herramienta autenticada de edición. El plan no se marcó como actualizado. Este bloqueo queda también reflejado en `openspec/changes/validacion-visual-bootstrap/tasks.md`.

## Implementación web inicial — 2026-11-16

La web se verificó desde `/Volumes/Server/Proyectos/energyhub-universal/apps/web` con Node 22.23.4 y pnpm 11.0.8. Se añadió Vitest 5.1.0 y `@vitest/coverage-v8` 5.1.0; el lockfile de `apps/web` quedó actualizado.

- `pnpm test`: 12/12 pruebas.
- `pnpm exec vitest run --coverage`: statements 100%, branches 95.83%, functions 100%, lines 100%.
- `pnpm exec tsc --noEmit`: correcto.
- `pnpm lint`: correcto.
- `pnpm build`: correcto; Next.js 16.4.6 produjo `/` como prerender parcial con el metadato de antigüedad en streaming dentro de `Suspense`.
- Revisión del navegador integrado: viewports 366×812 y 1096×1265, sin overflow horizontal, consola sin errores, teclado con skip link y foco visible.
- Lighthouse final en móvil y escritorio: accesibilidad 100, buenas prácticas 100, SEO 100.

La evidencia está en `docs/evidence/implementacion-web-inicial-2026-11-16/`. Las pruebas en teléfonos físicos siguen pendientes. La actualización del plan maestro de Notion se confirmó al cierre de esta ejecución; los intentos anteriores fallaron con `ECONNRESET`.

La separación pantalla–datos está en `apps/web/app/site/provider.ts`; la fixture sintética está en `example.ts`; los tipos de lecturas, dispositivos y frescura están en `types.ts`. La siguiente fase backend está diseñada, sin implementación, en `docs/architecture/backend-next-phase.md` y `contracts/openapi/next-phase-proposal.yaml`.

## Endurecimiento del backend y toolchain — 2026-11-18

- Toolchain del backend: Go 1.27.4 (antes 1.27.1). `backend/go.mod` fija la versión y CI la toma con `go-version-file: backend/go.mod`, así que no circulan dos versiones. `go mod tidy` no cambió dependencias: el `go.sum` sigue igual.
- Verificación en verde con la toolchain nueva: `go test ./...`, `go vet ./...`, `go build ./...`, `go mod verify` y `go test -race ./...`.
- Integración real contra PostgreSQL 17.11 (Homebrew, `127.0.0.1:55432`) en verde: aislamiento de tenants, frontera administrativa con auditoría, conflicto de creación duplicada, rollback de `000004` y `000003`, propiedad de los objetos en `energyhub_owner` y migración ejecutada por un rol no superusuario.
- Cobertura medida: `authn` 95.3%, `authz` 100%, `httpapi` 95.9%, `site` 84.6%, `devoidc` 82.1%, `storage/postgres` 98.2% (sin la suite de integración).
- Se mantiene el alcance: sin pruebas en teléfonos físicos, sin billing, sin telemetría, sin conectores de hardware, sin datos reales y sin superficie HTTP de administración.
- Revisión de código y de seguridad posterior al endurecimiento: se añadió la migración `000005` para cerrar el hueco de `TRUNCATE` (que el RLS no cubre) y reafirmar la propiedad de las funciones administrativas sobre `energyhub_owner`; el bootstrap ahora resuelve la pertenencia transitiva de roles y falla cerrado si un rol alcanza los grupos administrativos por una cadena o si la credencial administrativa se comparte; la verificación de rol de la API cubre `INSERT`, `UPDATE`, `DELETE` y `TRUNCATE` sobre las tres tablas protegidas; `cmd/migrate` comprueba el rol effective de la sesión y no solo el userinfo de la URL; la ruta protegida marca `Vary: Authorization` también en sus errores; el emisor de desarrollo rechaza el contenido posterior al primer objeto JSON y responde `413` a un cuerpo demasiado grande.
- Higiene del cluster de pruebas: la suite de integración liberaba los privilegios del rol temporal solo después de eliminar la base de prueba, así que el rol con `TRUNCATE` no se podía borrar y el fallo se silenciaba con `t.Logf`. El cluster acumulaba roles `energyhub_it_truncate_*`. Ahora los privilegios se liberan con `DROP OWNED BY ... CASCADE` antes de borrar el rol, y la limpieza usa la URL explícita del superusuario en vez de reconstruirla desde la configuración del pool. Comprobado con `pg_roles` y `pg_database`: cero roles y cero bases residuales.
- Redaction de credenciales: `cmd/api`, `cmd/membership` y `cmd/migrate` quitan la contraseña del DSN de sus errores de conexión antes de mostrarlos. El driver de PostgreSQL incluye la cadena original sin filtrar en `pgconn.ParseConfigError`, y ese error llega al log de arranque; la credencial de migración es la más sensible del sistema. Cubierto con pruebas unitarias de `postgres.RedactConnectionError`.
- Lista de procesos: el stack local exportaba el DSN del runtime como argumento de `env`, así que era visible con `ps`. Ahora el API arranca en un subshell que elimina `ADMIN_DATABASE_URL` y `MIGRATION_DATABASE_URL` del entorno heredado y exporta el DSN del runtime, de modo que la lista de procesos no muestra ninguna credencial.
- Integración en CI: el job de backend levanta un PostgreSQL 17 como servicio en `127.0.0.1:55432` y ejecuta `go test ./internal/storage/postgres -run TestIntegration` con `TEST_DATABASE_URL`. Antes, la suite que prueba el aislamiento de tenants y la frontera administrativa solo corría en la máquina del desarrollador, y una regresión de permisos habría llegado al despliegue sin que nada la detuviera.

## Endpoints de sitios y dispositivos — 2026-11-20

- Implementados los 4 endpoints de `contracts/openapi/next-phase-proposal.yaml`: `GET /v1/me`, `GET /v1/organizations/{organizationId}/sites`, `GET /v1/organizations/{organizationId}/sites/{siteId}/overview` y `GET /v1/organizations/{organizationId}/sites/{siteId}/devices` (contrato cumplido exactamente; la propuesta no se modificó).
- Migración `000007` (`sites` y `site_devices`, RLS forzado por membresía, funciones admin `app.create_site` / `app.create_site_device` con auditoría); reader Postgres con `app.current_subject` en transacción; handlers con sobre de error común, `no-store` y `Vary: Authorization`; CLI de admin `create-site` / `create-device` (UUID v4 con stdlib).
- Sin telemetría: el overview devuelve `readings: []` y `updatedAt: null` hasta que exista origen de datos real. Creación solo por CLI; sin mutaciones HTTP.
- Verificación en verde: `go test ./...` 297 casos, `go vet`, `go build`, `go test -race`, integración real 15/15 (PostgreSQL 17 local) y E2E 12/12 contra el stack local (200 con datos, 401/403/404). Evidencia en `docs/evidence/backend-sites-2026-11-20/`.
- Desviación documentada: el `Reader` de `internal/site` ganó un parámetro `subject` para fijar el contexto RLS; corrección del orden de argumentos del writer para llamar a las funciones admin en su orden declarado. Commits locales `48bd01d`, `faecfd7`, `cf2b5bc`, `78ba01b`, `597f96f`, `e7fa732` (+ docs), sin push.
- Queda: telemetría/lecturas, conectar las 3 apps a la API real, pruebas en teléfono físico y el push.
