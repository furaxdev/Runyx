package dev.furax.runyx

import android.graphics.Bitmap
import com.google.zxing.BinaryBitmap
import com.google.zxing.MultiFormatReader
import com.google.zxing.RGBLuminanceSource
import com.google.zxing.common.HybridBinarizer

/**
 * A paired PC's address + device token (item 26), decoded from the QR
 * code Runyx's `/api/runyx/pairing/qrcode` endpoint renders, matching
 * internal/pairing.Info.URL()'s "runyx://pair?d=<base64>" format.
 */
data class PairedHost(val host: String, val port: Int, val token: String) {
    val baseUrl: String get() = "http://$host:$port"
}

/** Thrown when a decoded string isn't a valid Runyx pairing URL. */
class InvalidPairingCodeException(message: String) : Exception(message)

/**
 * Decodes a `runyx://pair?d=<base64 of "host:port:token">` URL, exactly
 * matching the encoding in internal/pairing.Info.URL() (Go side).
 */
fun parsePairingUrl(url: String): PairedHost {
    val prefix = "runyx://pair?d="
    if (!url.startsWith(prefix)) {
        throw InvalidPairingCodeException("not a Runyx pairing code: $url")
    }
    val encoded = url.removePrefix(prefix)
    val decoded = try {
        String(decodeBase64UrlNoPadding(encoded))
    } catch (e: Exception) {
        throw InvalidPairingCodeException("could not decode pairing payload: ${e.message}")
    }
    val parts = decoded.split(":")
    if (parts.size != 3) {
        throw InvalidPairingCodeException("malformed pairing payload: $decoded")
    }
    val port = parts[1].toIntOrNull()
        ?: throw InvalidPairingCodeException("malformed port in pairing payload: ${parts[1]}")
    val host = parts[0]
    if (!isPrivateOrLocalHost(host)) {
        throw InvalidPairingCodeException(
            "refusing to pair with non-private host \"$host\": Runyx only talks " +
                "cleartext HTTP to a PC on your own LAN (see docs/DECISIONS.md, item 23)"
        )
    }
    return PairedHost(host = host, port = port, token = parts[2])
}

/**
 * Reports whether host is something the app allows talking plain HTTP
 * to: a private IPv4 range (RFC 1918), loopback, or an mDNS ".local"
 * name. Android's network_security_config.xml can declare cleartext
 * allowed for literal domain names (and the ".local" suffix), but it has
 * no CIDR/subnet syntax for IP ranges — so for the common case of
 * pairing with a bare LAN IP, this app-level check is what actually
 * enforces "LAN only", matching what network_security_config.xml
 * declares for name-based hosts.
 */
fun isPrivateOrLocalHost(host: String): Boolean {
    if (host.endsWith(".local", ignoreCase = true)) return true
    val octets = host.split(".").mapNotNull { it.toIntOrNull() }
    if (octets.size != 4 || octets.any { it !in 0..255 }) return false
    val (a, b, _, _) = octets
    return when {
        a == 10 -> true // 10.0.0.0/8
        a == 172 && b in 16..31 -> true // 172.16.0.0/12
        a == 192 && b == 168 -> true // 192.168.0.0/16
        a == 127 -> true // 127.0.0.0/8 (loopback, same-device testing)
        else -> false
    }
}

/**
 * Decodes unpadded URL-safe base64 (Go's base64.RawURLEncoding) using
 * java.util.Base64 (available since API 26, our minSdk) instead of
 * android.util.Base64, so this logic is also testable as a plain JVM
 * unit test (android.* classes are unavailable/stubbed under
 * src/test without Robolectric).
 */
private fun decodeBase64UrlNoPadding(s: String): ByteArray {
    val padded = when (s.length % 4) {
        2 -> "$s=="
        3 -> "$s="
        else -> s
    }
    return java.util.Base64.getUrlDecoder().decode(padded)
}

/**
 * Decodes a QR code from a captured photo (see the camera-intent flow in
 * PairingScreen) using zxing's pure-Java decoder — no CameraX/live-scan
 * dependency needed for this first pass.
 */
fun decodeQRCode(bitmap: Bitmap): String? {
    val width = bitmap.width
    val height = bitmap.height
    val pixels = IntArray(width * height)
    bitmap.getPixels(pixels, 0, width, 0, 0, width, height)
    val source = RGBLuminanceSource(width, height, pixels)
    val binaryBitmap = BinaryBitmap(HybridBinarizer(source))
    return try {
        MultiFormatReader().decode(binaryBitmap).text
    } catch (e: Exception) {
        null // no QR code found in the image
    }
}
