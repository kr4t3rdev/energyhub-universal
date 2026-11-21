import { NextResponse } from "next/server";

/**
 * Inicia el flujo de autenticación de Auth0 redirigiendo al usuario al
 * endpoint de autorización. El cliente debe configurar AUTH0_ISSUER,
 * AUTH0_AUDIENCE y NEXT_PUBLIC_AUTH0_CLIENT_ID en las variables de entorno.
 */
export async function GET(_request: Request) {
  void _request;
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

  // Generar un estado aleatorio para prevenir CSRF
  const state = crypto.randomUUID();

  // Guardar el estado en una cookie para validarlo en el callback
  const redirectUrl = new URL(`${issuer}/authorize`);
  redirectUrl.searchParams.set("response_type", "code");
  redirectUrl.searchParams.set("client_id", clientId);
  redirectUrl.searchParams.set("redirect_uri", callbackUrl);
  redirectUrl.searchParams.set("scope", "openid profile email");
  redirectUrl.searchParams.set("audience", audience);
  redirectUrl.searchParams.set("state", state);
  redirectUrl.searchParams.set("prompt", "login");

  const isSecure = process.env.NODE_ENV === "production";
  const response = NextResponse.redirect(redirectUrl);
  response.headers.set(
    "Set-Cookie",
    `auth0_state=${state}; HttpOnly; Path=/; Max-Age=600; SameSite=Lax${
      isSecure ? "; Secure" : ""
    }`,
  );

  return response;
}
