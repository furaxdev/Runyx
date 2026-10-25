package dev.furax.runyx

import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import kotlinx.coroutines.launch

/** One message in the on-screen transcript. */
data class UiMessage(val role: String, val content: String)

/**
 * Holds pairing state and drives chat against the paired PC. Kept as a
 * single ViewModel for this first pass rather than a full
 * navigation-graph + per-screen ViewModel setup, since the app is still
 * a small number of screens (item 27's camera/share-sheet/widget/tile
 * additions would be natural places to split it up further).
 */
class RunyxViewModel : ViewModel() {
    var paired by mutableStateOf<PairedHost?>(null)
    var pairingError by mutableStateOf<String?>(null)

    var messages by mutableStateOf(listOf<UiMessage>())
    var liveReply by mutableStateOf("")
    var isStreaming by mutableStateOf(false)
    var selectedModel by mutableStateOf<String?>(null)
    var availableModels by mutableStateOf(listOf<String>())
    var chatError by mutableStateOf<String?>(null)

    var isRecording by mutableStateOf(false)
    var isTranscribing by mutableStateOf(false)

    private lateinit var store: PairingStore

    fun init(store: PairingStore) {
        this.store = store
        paired = store.load()
        paired?.let { loadModels(it) }
    }

    fun pairWithCode(rawCode: String) {
        pairingError = null
        try {
            val host = parsePairingUrl(rawCode)
            store.save(host)
            paired = host
            loadModels(host)
        } catch (e: InvalidPairingCodeException) {
            pairingError = e.message
        }
    }

    fun unpair() {
        store.clear()
        paired = null
        messages = emptyList()
    }

    private fun loadModels(host: PairedHost) = viewModelScope.launch {
        try {
            val client = RunyxClient(host.baseUrl, host.token)
            availableModels = client.models()
            if (selectedModel == null) selectedModel = availableModels.firstOrNull()
        } catch (e: Exception) {
            chatError = "Could not reach the paired PC: ${e.message}"
        }
    }

    /** Sends a user message (optionally pre-filled from a Share Sheet
     * intent, item 27) and streams the assistant's reply. */
    fun send(text: String) {
        val host = paired ?: return
        val model = selectedModel ?: return
        if (text.isBlank()) return

        messages = messages + UiMessage("user", text)
        isStreaming = true
        liveReply = ""
        chatError = null

        viewModelScope.launch {
            try {
                val client = RunyxClient(host.baseUrl, host.token)
                val history = messages.map { RunyxClient.ChatMessage(it.role, it.content) }
                client.streamChat(model, history) { delta ->
                    liveReply += delta
                }
                messages = messages + UiMessage("assistant", liveReply)
            } catch (e: Exception) {
                chatError = "Chat request failed: ${e.message}"
            } finally {
                isStreaming = false
                liveReply = ""
            }
        }
    }

    /** Called once a voice recording has been transcribed (item 25/27). */
    fun onTranscribed(text: String) {
        isTranscribing = false
        if (text.isNotBlank()) send(text)
    }
}
