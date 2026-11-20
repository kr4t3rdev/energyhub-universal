package com.energyhub.universal.site

import com.energyhub.universal.BuildConfig

/**
 * Errores del cliente de API de EnergyHub Universal.
 */
sealed class ApiClientError : Exception() {
    object BaseUrlNotConfigured : ApiClientError()
    object InvalidResponse : ApiClientError()
    class RequestFailed(val statusCode: Int, val body: String) : ApiClientError()
    class DecodingFailed(val source: Throwable) : ApiClientError()
}

/**
 * Cliente HTTP mínimo para la API de EnergyHub Universal.
 *
 * La URL base se lee de `BuildConfig.API_BASE_URL`, que se define en
 * `build.gradle.kts`. Cuando está vacía, el cliente opera en modo sin conexión.
 *
 * El token de autenticación (Bearer) se inyecta desde fuera. El cliente nunca
 * almacena ni lee tokens por su cuenta.
 */
class ApiClient private constructor(
    private val baseUrl: String,
    private val authToken: String?,
) {

    constructor(baseUrl: String) : this(baseUrl, null)

    suspend fun get(path: String): String {
        val url = java.net.URL("$baseUrl$path")
        val connection = url.openConnection() as java.net.HttpURLConnection
        connection.requestMethod = "GET"
        connection.setRequestProperty("Accept", "application/json")
        authToken?.let { connection.setRequestProperty("Authorization", "Bearer $it") }

        val responseCode = connection.responseCode
        val responseBody = connection.inputStream.bufferedReader().use { it.readText() }

        if (responseCode !in 200..299) {
            throw ApiClientError.RequestFailed(responseCode, responseBody)
        }

        return responseBody
    }

    companion object {
        fun create(authToken: String? = null): ApiClient? {
            val baseUrl = BuildConfig.API_BASE_URL
            return if (baseUrl.isNotBlank()) ApiClient(baseUrl, authToken) else null
        }
    }
}

/**
 * Lee la URL base de la API desde la configuración de build.
 */
fun readApiBaseUrl(): String = BuildConfig.API_BASE_URL
