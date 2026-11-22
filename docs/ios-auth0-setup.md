# Configuración de Auth0 para iOS

La app usa Auth0.swift con Authorization Code + PKCE. Es una aplicación nativa
pública: no configurar ni distribuir un Client Secret.

## Configuración del cliente en el proyecto

`apps/ios/project.yml` define el dominio y los identificadores públicos que se
empaquetan en el `Info.plist`. El Client ID, el dominio y el API Identifier no
son secretos; los access/refresh tokens nunca se guardan en `UserDefaults`.
Auth0.swift administra las credenciales en Keychain.

El Bundle ID es `com.energyhub.universal`. En la aplicación Native del tenant,
registrar esta URL personalizada en **Allowed Callback URLs** y
**Allowed Logout URLs**:

```text
com.energyhub.universal://dev-4nf302ypyiy2lxjl.us.auth0.com/ios/com.energyhub.universal/callback
```

La app registra el esquema `com.energyhub.universal` en su `Info.plist`.
La URL HTTPS opcional que Auth0.swift admite para iOS 17.4+ es:

```text
https://dev-4nf302ypyiy2lxjl.us.auth0.com/ios/com.energyhub.universal/callback
```

## API y renovación

La app solicita el audience `https://api.energyhub.kr4t3rdev.xyz` y los scopes
`openid profile email offline_access`. Para renovación automática, habilitar
**Allow Offline Access** en la API de Auth0. El backend debe aceptar tokens
emitidos por el mismo issuer y para ese audience.

Después de cambiar `project.yml`, regenerar el proyecto con XcodeGen y compilar
el scheme `EnergyHub`.
