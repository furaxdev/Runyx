package dev.furax.runyx

import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import org.json.JSONArray
import org.json.JSONObject
import java.io.BufferedReader
import java.io.InputStreamReader
import java.io.OutputStreamWriter
import java.net.HttpURLConnection
import java.net.URL
import javax.net.ssl.HttpsURLConnection

/**
 * Minimal HTTP client for a paired Runyx daemon: chat (streaming via the
 * OpenAI-compatible endpoint), the dashboard, and voice transcription.
 * Deliberately dependency-free (no OkHttp/Retrofit) to keep the app
 * buildable without extra network libraries in this first pass.
 */
class RunyxClient(private val baseUrl: String, private val token: String?) {

    /** One chat message in Runyx's/OpenAI's wire shape. */
    data class ChatMessage(val role: String, val content: String)

    /**
     * Streams a chat completion, invoking onDelta for each token chunk as
     * it arrives (Server-Sent-Events parsing of "data: {...}" lines).
     */
    suspend fun streamChat(model: String, messages: List<ChatMessage>, onDelta: (String) -> Unit) =
        withContext(Dispatchers.IO) {
            val url = URL("$baseUrl/v1/chat/completions")
            val conn = openConnection(url, "POST")
            conn.doOutput = true

            val payload = JSONObject().apply {
                put("model", model)
                put("stream", true)
                put("messages", JSONArray().apply {
                    messages.forEach { m ->
                        put(JSONObject().apply { put("role", m.role); put("content", m.content) })
                    }
                })
            }
            OutputStreamWriter(conn.outputStream).use { it.write(payload.toString()) }

            BufferedReader(InputStreamReader(conn.inputStream)).use { reader ->
                reader.lineSequence().forEach { line ->
                    if (!line.startsWith("data: ")) return@forEach
                    val data = line.removePrefix("data: ")
                    if (data == "[DONE]") return@forEach
                    runCatching {
                        val chunk = JSONObject(data)
                        val choices = chunk.optJSONArray("choices") ?: return@runCatching
                        if (choices.length() == 0) return@runCatching
                        val delta = choices.getJSONObject(0).optJSONObject("delta") ?: return@runCatching
                        val content = delta.optString("content", "")
                        if (content.isNotEmpty()) onDelta(content)
                    }
                }
            }
        }

    /** Fetches the dashboard stats (used on the home screen). */
    suspend fun dashboard(): JSONObject = withContext(Dispatchers.IO) {
        val conn = openConnection(URL("$baseUrl/api/runyx/dashboard"), "GET")
        JSONObject(conn.inputStream.bufferedReader().readText())
    }

    /** Lists installed models, for the model picker. */
    suspend fun models(): List<String> = withContext(Dispatchers.IO) {
        val conn = openConnection(URL("$baseUrl/v1/models"), "GET")
        val body = JSONObject(conn.inputStream.bufferedReader().readText())
        val data = body.optJSONArray("data") ?: JSONArray()
        (0 until data.length()).map { data.getJSONObject(it).getString("id") }
    }

    private fun openConnection(url: URL, method: String): HttpURLConnection {
        val conn = url.openConnection() as HttpURLConnection
        conn.requestMethod = method
        conn.connectTimeout = 10_000
        conn.readTimeout = 60_000
        conn.setRequestProperty("Content-Type", "application/json")
        if (!token.isNullOrEmpty()) {
            conn.setRequestProperty("Authorization", "Bearer $token")
        }
        if (conn is HttpsURLConnection) {
            // Pairing (item 26) uses a TLS-pinned connection once the PC's
            // certificate is captured at pairing time; that pinning isn't
            // implemented in this first pass — see docs/BLOCKED.md. Plain
            // HTTP to a LAN address is what pairing currently produces.
        }
        return conn
    }
}
