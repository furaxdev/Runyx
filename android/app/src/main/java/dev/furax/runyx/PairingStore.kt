package dev.furax.runyx

import android.content.Context

/** Persists the paired PC's connection details across app restarts. */
class PairingStore(context: Context) {
    private val prefs = context.getSharedPreferences("runyx_pairing", Context.MODE_PRIVATE)

    fun load(): PairedHost? {
        val host = prefs.getString("host", null) ?: return null
        val port = prefs.getInt("port", -1)
        val token = prefs.getString("token", null) ?: return null
        if (port <= 0) return null
        return PairedHost(host, port, token)
    }

    fun save(paired: PairedHost) {
        prefs.edit()
            .putString("host", paired.host)
            .putInt("port", paired.port)
            .putString("token", paired.token)
            .apply()
    }

    fun clear() {
        prefs.edit().clear().apply()
    }
}
