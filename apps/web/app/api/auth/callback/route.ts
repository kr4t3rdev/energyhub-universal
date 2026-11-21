import { NextResponse } from "next/server";

/**
 * Recibe el código de autorización de Auth0, lo intercambia por un access
 * token y devuelve una página HTML que guarda el token en localStorage y
 * redirige a la página principal.
 */
export async function GET(request: Request) {
  const { searchParams } = new URL(request.url);
  const code = searchParams.get("code");
  const state = searchParams.get("state");
  const error = searchParams.get("error");
  const errorDescription = searchParams.get("error_description");

  if (error) {
    return NextResponse.json(
      { error: errorDescription || error },
      { status: 400 },
    );
  }

  if (!code || !state) {
    return NextResponse.json(
      { error: "Parámetros de autorización incompletos" },
      { status: 400 },
    );
  }

  // Validar el estado para prevenir CSRF
  const cookieHeader = request.headers.get("cookie") || "";
  const storedState = cookieHeader
    .split(";")
    .map((c) => c.trim().split("="))
    .find(([key]) => key === "auth0_state")?.[1];

  if (!storedState || storedState !== state) {
    return NextResponse.json(
      { error: "Estado de autenticación inválido" },
      { status: 400 },
    );
  }

  const issuer = process.env.AUTH0_ISSUER;
  const audience = process.env.AUTH0_AUDIENCE;
  const clientId = process.env.NEXT_PUBLIC_AUTH0_CLIENT_ID;
  const callbackUrl = process.env.NEXT_PUBLIC_AUTH0_CALLBACK_URL;

  if (!issuer || !audience || !clientId || !callbackUrl) {
    return NextResponse.json(
      { error: "Auth0 no está configurado en el servidor" },
      { status: 500 },
    );
  }

  try {
    // Intercambiar el código por un access token
    const tokenResponse = await fetch(`${issuer}/oauth/token`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        grant_type: "authorization_code",
        client_id: clientId,
        code,
        redirect_uri: callbackUrl,
        audience,
      }),
    });

    if (!tokenResponse.ok) {
      const errorBody = await tokenResponse.text();
      return NextResponse.json(
        { error: `Error al obtener token: ${errorBody}` },
        { status: tokenResponse.status },
      );
    }

    const tokens = await tokenResponse.json();
    const accessToken = tokens.access_token;
    const idToken = tokens.id_token;
    const expiresIn = tokens.expires_in;

    if (!accessToken) {
      return NextResponse.json(
        { error: "No se recibió access token" },
        { status: 500 },
      );
    }

    // Devolver una página HTML que guarde el token en localStorage y redirija
    const html = `
<!DOCTYPE html>
<html>
<head>
  <title>Iniciando sesión...</title>
  <meta charset="utf-8" />
</head>
<body>
  <script>
    // Guardar el token en localStorage
    localStorage.setItem("energyhub_token", "${accessToken.replace(/"/g, '\\"')}");
    localStorage.setItem("energyhub_id_token", "${idToken?.replace(/"/g, '\\"') || ""}");
    localStorage.setItem("energyhub_expires_at", "${Date.now() + (expiresIn || 3600) * 1000}");
    // Redirigir a la página principal
    window.location.href = "/";
  </script>
  <p>Iniciando sesión, redirigiendo...</p>
</body>
</html>
    `;

    const response = new NextResponse(html, {
      headers: { "Content-Type": "text/html; charset=utf-8" },
    });
    // Eliminar la cookie de estado
    response.headers.set(
      "Set-Cookie",
      "auth0_state=; HttpOnly; Path=/; Max-Age=0",
    );
    return response;
  } catch (_error) {
    void _error;
    return NextResponse.json(
      { error: "Error interno del servidor" },
      { status: 500 },
    );
  }
}
