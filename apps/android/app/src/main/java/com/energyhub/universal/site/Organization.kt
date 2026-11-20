package com.energyhub.universal.site

/**
 * Organización del sujeto autenticado, tal como la devuelve
 * GET /v1/organizations (o /v1/me) del backend.
 */
data class Organization(
    val id: String,
    val name: String,
    val role: String,
)

/**
 * Respuesta actual del usuario autenticado.
 */
data class CurrentUser(
    val subject: String,
    val organizations: List<Organization>,
)
