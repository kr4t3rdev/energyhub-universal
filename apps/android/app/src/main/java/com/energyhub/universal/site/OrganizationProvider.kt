package com.energyhub.universal.site

import org.json.JSONArray
import org.json.JSONObject

/**
 * Proveedor de organizaciones que llama a la API real.
 *
 * Cuando la URL base de la API no está configurada, devuelve una lista vacía
 * para que la UI pueda decidir quedarse en modo demo.
 */
class OrganizationApiProvider(private val authToken: String? = null) {

    private val client: ApiClient? = ApiClient.create(authToken)

    suspend fun getOrganizations(): List<Organization> {
        val client = this.client ?: return emptyList()

        val json = client.get("/v1/organizations")
        val currentUser = JSONObject(json)
        val subject = currentUser.getString("subject")
        val organizationsArray = currentUser.getJSONArray("organizations")

        val organizations = mutableListOf<Organization>()
        for (i in 0 until organizationsArray.length()) {
            val orgJson = organizationsArray.getJSONObject(i)
            organizations.add(
                Organization(
                    id = orgJson.getString("id"),
                    name = orgJson.getString("name"),
                    role = orgJson.getString("role"),
                )
            )
        }

        return organizations
    }
}

/**
 * Indica si la aplicación está configurada para hablar con la API real.
 */
fun isApiConfigured(): Boolean = BuildConfig.API_BASE_URL.isNotBlank()
