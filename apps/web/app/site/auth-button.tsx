"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";

/**
 * Componente que muestra un botón de "Iniciar sesión" cuando no hay token,
 * o "Cerrar sesión" cuando hay uno válido.
 *
 * El token se lee de localStorage (clave: energyhub_token).
 */
export function AuthButton() {
  const router = useRouter();
  const [isAuthenticated, setIsAuthenticated] = useState(false);

  useEffect(() => {
    const checkAuth = () => {
      const token = localStorage.getItem("energyhub_token");
      const expiresAt = localStorage.getItem("energyhub_expires_at");
      setIsAuthenticated(
        !!token &&
          !!expiresAt &&
          Date.now() < parseInt(expiresAt, 10),
      );
    };

    checkAuth();
    window.addEventListener("storage", checkAuth);
    return () => window.removeEventListener("storage", checkAuth);
  }, []);

  const handleLogin = () => {
    const callbackUrl = process.env.NEXT_PUBLIC_AUTH0_CALLBACK_URL;
    if (!callbackUrl) {
      console.error("NEXT_PUBLIC_AUTH0_CALLBACK_URL no está configurado");
      return;
    }
    router.push("/api/auth/login");
  };

  const handleLogout = () => {
    localStorage.removeItem("energyhub_token");
    localStorage.removeItem("energyhub_id_token");
    localStorage.removeItem("energyhub_expires_at");
    setIsAuthenticated(false);
    router.push("/");
  };

  return (
    <div className="auth-controls">
      {isAuthenticated ? (
        <button
          onClick={handleLogout}
          className="auth-button auth-button-logout"
        >
          Cerrar sesión
        </button>
      ) : (
        <button onClick={handleLogin} className="auth-button auth-button-login">
          Iniciar sesión
        </button>
      )}
    </div>
  );
}
