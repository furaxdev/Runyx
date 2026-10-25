package dev.furax.runyx

import org.junit.Assert.assertEquals
import org.junit.Assert.assertThrows
import org.junit.Test

/**
 * Pure-JVM unit test (no Android runtime needed) for the pairing URL
 * format, checked against the exact same encoding the Go daemon side
 * produces (internal/pairing.Info.URL(), base64.RawURLEncoding of
 * "host:port:token").
 */
class PairingTest {
    @Test
    fun `parses a valid pairing URL`() {
        // base64.RawURLEncoding("192.168.1.50:11535:runyx_abc123")
        val url = "runyx://pair?d=MTkyLjE2OC4xLjUwOjExNTM1OnJ1bnl4X2FiYzEyMw"
        val host = parsePairingUrl(url)
        assertEquals("192.168.1.50", host.host)
        assertEquals(11535, host.port)
        assertEquals("runyx_abc123", host.token)
        assertEquals("http://192.168.1.50:11535", host.baseUrl)
    }

    @Test
    fun `rejects a non-runyx URL`() {
        assertThrows(InvalidPairingCodeException::class.java) {
            parsePairingUrl("https://example.com")
        }
    }

    @Test
    fun `rejects malformed base64`() {
        assertThrows(InvalidPairingCodeException::class.java) {
            parsePairingUrl("runyx://pair?d=not-valid-base64!!!")
        }
    }

    @Test
    fun `rejects a payload missing fields`() {
        // decodes to "host-only", missing port and token
        assertThrows(InvalidPairingCodeException::class.java) {
            parsePairingUrl("runyx://pair?d=aG9zdC1vbmx5")
        }
    }

    @Test
    fun `rejects a public-internet host`() {
        // base64.RawURLEncoding("8.8.8.8:11535:runyx_abc123")
        val url = "runyx://pair?d=OC44LjguODoxMTUzNTpydW55eF9hYmMxMjM"
        assertThrows(InvalidPairingCodeException::class.java) {
            parsePairingUrl(url)
        }
    }

    @Test
    fun `isPrivateOrLocalHost accepts RFC1918 ranges, loopback and mDNS names`() {
        for (host in listOf("10.1.2.3", "172.16.0.1", "172.31.255.255", "192.168.0.1", "127.0.0.1", "mypc.local")) {
            assert(isPrivateOrLocalHost(host)) { "$host should be allowed" }
        }
    }

    @Test
    fun `isPrivateOrLocalHost rejects public addresses and garbage`() {
        for (host in listOf("8.8.8.8", "1.1.1.1", "172.32.0.1", "example.com", "not-an-ip")) {
            assert(!isPrivateOrLocalHost(host)) { "$host should be rejected" }
        }
    }
}
