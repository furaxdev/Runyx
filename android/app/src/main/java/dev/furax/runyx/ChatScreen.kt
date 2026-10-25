package dev.furax.runyx

import android.Manifest
import android.content.pm.PackageManager
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Mic
import androidx.compose.material.icons.automirrored.filled.Send
import androidx.compose.material.icons.filled.Stop
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.unit.dp
import androidx.core.content.ContextCompat
import kotlinx.coroutines.launch

/** Main chat screen: text input, push-to-talk mic, and streamed replies. */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun ChatScreen(viewModel: RunyxViewModel, prefillText: String?) {
    val context = LocalContext.current
    val scope = rememberCoroutineScope()
    var input by remember { mutableStateOf(prefillText ?: "") }
    val recorder = remember { VoiceRecorder(context) }

    val micPermission = rememberLauncherForActivityResult(ActivityResultContracts.RequestPermission()) { granted ->
        if (granted) startRecording(viewModel, recorder)
    }

    LaunchedEffect(prefillText) {
        if (!prefillText.isNullOrBlank()) {
            viewModel.send(prefillText)
            input = ""
        }
    }

    Column(Modifier.fillMaxSize()) {
        TopAppBar(
            title = { Text(viewModel.selectedModel ?: "Runyx") },
            actions = {
                TextButton(onClick = { viewModel.unpair() }) { Text("Disconnect") }
            }
        )

        LazyColumn(modifier = Modifier.weight(1f).padding(12.dp)) {
            items(viewModel.messages) { msg ->
                Row(
                    Modifier.fillMaxWidth().padding(vertical = 4.dp),
                    horizontalArrangement = if (msg.role == "user") Arrangement.End else Arrangement.Start
                ) {
                    Card {
                        Text(msg.content, Modifier.padding(10.dp))
                    }
                }
            }
            if (viewModel.isStreaming) {
                item {
                    Card { Text(viewModel.liveReply + " ▌", Modifier.padding(10.dp)) }
                }
            }
        }

        viewModel.chatError?.let {
            Text(it, color = MaterialTheme.colorScheme.error, modifier = Modifier.padding(8.dp))
        }

        Row(
            Modifier.fillMaxWidth().padding(8.dp),
            verticalAlignment = Alignment.CenterVertically
        ) {
            IconButton(onClick = {
                if (viewModel.isRecording) {
                    scope.launch { stopRecording(viewModel, recorder) }
                } else {
                    requestMicAndRecord(context, micPermission, viewModel, recorder)
                }
            }) {
                if (viewModel.isRecording) {
                    Icon(Icons.Default.Stop, contentDescription = "Stop recording")
                } else {
                    Icon(Icons.Default.Mic, contentDescription = "Start voice input")
                }
            }
            OutlinedTextField(
                value = input,
                onValueChange = { input = it },
                modifier = Modifier.weight(1f),
                placeholder = { Text("Type a message...") }
            )
            IconButton(onClick = {
                viewModel.send(input)
                input = ""
            }) {
                Icon(Icons.AutoMirrored.Filled.Send, contentDescription = "Send message")
            }
        }
    }
}

private fun requestMicAndRecord(
    context: android.content.Context,
    launcher: androidx.activity.result.ActivityResultLauncher<String>,
    viewModel: RunyxViewModel,
    recorder: VoiceRecorder
) {
    val granted = ContextCompat.checkSelfPermission(context, Manifest.permission.RECORD_AUDIO) ==
        PackageManager.PERMISSION_GRANTED
    if (granted) {
        startRecording(viewModel, recorder)
    } else {
        launcher.launch(Manifest.permission.RECORD_AUDIO)
    }
}

private fun startRecording(viewModel: RunyxViewModel, recorder: VoiceRecorder) {
    try {
        recorder.start()
        viewModel.isRecording = true
    } catch (e: Exception) {
        viewModel.chatError = "Microphone: ${e.message}"
    }
}

private suspend fun stopRecording(viewModel: RunyxViewModel, recorder: VoiceRecorder) {
    val host = viewModel.paired ?: return
    viewModel.isRecording = false
    viewModel.isTranscribing = true
    try {
        val text = recorder.stopAndTranscribe(host)
        viewModel.onTranscribed(text)
    } catch (e: Exception) {
        viewModel.isTranscribing = false
        viewModel.chatError = "Transcription: ${e.message}"
    }
}
