package dev.furax.runyx

import android.Manifest
import android.content.pm.PackageManager
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.animation.AnimatedVisibility
import androidx.compose.animation.core.LinearEasing
import androidx.compose.animation.core.RepeatMode
import androidx.compose.animation.core.animateFloat
import androidx.compose.animation.core.infiniteRepeatable
import androidx.compose.animation.core.rememberInfiniteTransition
import androidx.compose.animation.core.tween
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.lazy.rememberLazyListState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.Send
import androidx.compose.material.icons.filled.Mic
import androidx.compose.material.icons.filled.Stop
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
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
    val listState = rememberLazyListState()

    val micPermission = rememberLauncherForActivityResult(ActivityResultContracts.RequestPermission()) { granted ->
        if (granted) startRecording(viewModel, recorder)
    }

    LaunchedEffect(prefillText) {
        if (!prefillText.isNullOrBlank()) {
            viewModel.send(prefillText)
            input = ""
        }
    }

    // Keep the transcript pinned to the newest message/streaming token,
    // the way a chat app is expected to behave.
    LaunchedEffect(viewModel.messages.size, viewModel.liveReply, viewModel.isStreaming) {
        val target = viewModel.messages.size + if (viewModel.isStreaming) 1 else 0
        if (target > 0) listState.animateScrollToItem(target - 1)
    }

    Column(Modifier.fillMaxSize()) {
        TopAppBar(
            title = {
                Column {
                    Text(
                        viewModel.selectedModel ?: "Runyx",
                        style = MaterialTheme.typography.titleMedium,
                        maxLines = 1,
                    )
                    Text(
                        "Connected",
                        style = MaterialTheme.typography.labelSmall,
                        color = MaterialTheme.colorScheme.primary,
                    )
                }
            },
            actions = {
                TextButton(onClick = { viewModel.unpair() }) { Text("Disconnect") }
            },
            colors = TopAppBarDefaults.topAppBarColors(
                containerColor = MaterialTheme.colorScheme.surface,
            ),
        )
        HorizontalDivider()

        LazyColumn(
            state = listState,
            modifier = Modifier.weight(1f).fillMaxWidth(),
            contentPadding = PaddingValues(horizontal = 12.dp, vertical = 12.dp),
            verticalArrangement = Arrangement.spacedBy(10.dp),
        ) {
            items(viewModel.messages) { msg ->
                MessageBubble(role = msg.role, text = msg.content)
            }
            if (viewModel.isStreaming) {
                item {
                    if (viewModel.liveReply.isEmpty()) {
                        TypingIndicator()
                    } else {
                        MessageBubble(role = "assistant", text = viewModel.liveReply, streaming = true)
                    }
                }
            }
        }

        AnimatedVisibility(visible = viewModel.chatError != null, enter = fadeIn(), exit = fadeOut()) {
            Text(
                viewModel.chatError ?: "",
                color = MaterialTheme.colorScheme.error,
                style = MaterialTheme.typography.bodySmall,
                modifier = Modifier.padding(horizontal = 16.dp, vertical = 4.dp),
            )
        }

        HorizontalDivider()
        Row(
            Modifier
                .fillMaxWidth()
                .padding(horizontal = 10.dp, vertical = 8.dp),
            verticalAlignment = Alignment.CenterVertically
        ) {
            FilledTonalIconButton(
                onClick = {
                    if (viewModel.isRecording) {
                        scope.launch { stopRecording(viewModel, recorder) }
                    } else {
                        requestMicAndRecord(context, micPermission, viewModel, recorder)
                    }
                },
                colors = if (viewModel.isRecording) {
                    IconButtonDefaults.filledTonalIconButtonColors(
                        containerColor = MaterialTheme.colorScheme.errorContainer,
                        contentColor = MaterialTheme.colorScheme.onErrorContainer,
                    )
                } else {
                    IconButtonDefaults.filledTonalIconButtonColors()
                },
            ) {
                if (viewModel.isRecording) {
                    Icon(Icons.Default.Stop, contentDescription = "Stop recording")
                } else {
                    Icon(Icons.Default.Mic, contentDescription = "Start voice input")
                }
            }
            Spacer(Modifier.width(8.dp))
            OutlinedTextField(
                value = input,
                onValueChange = { input = it },
                modifier = Modifier.weight(1f),
                placeholder = { Text(if (viewModel.isTranscribing) "Transcribing…" else "Type a message...") },
                enabled = !viewModel.isTranscribing,
                shape = RoundedCornerShape(20.dp),
                maxLines = 5,
            )
            Spacer(Modifier.width(8.dp))
            val canSend = input.isNotBlank() && !viewModel.isStreaming
            FilledIconButton(
                onClick = {
                    viewModel.send(input)
                    input = ""
                },
                enabled = canSend,
            ) {
                Icon(Icons.AutoMirrored.Filled.Send, contentDescription = "Send message")
            }
        }
    }
}

@Composable
private fun MessageBubble(role: String, text: String, streaming: Boolean = false) {
    val isUser = role == "user"
    Row(
        Modifier.fillMaxWidth(),
        horizontalArrangement = if (isUser) Arrangement.End else Arrangement.Start,
    ) {
        Box(
            modifier = Modifier
                .widthIn(max = 300.dp)
                .clip(
                    RoundedCornerShape(
                        topStart = 16.dp,
                        topEnd = 16.dp,
                        bottomStart = if (isUser) 16.dp else 4.dp,
                        bottomEnd = if (isUser) 4.dp else 16.dp,
                    )
                )
                .background(
                    if (isUser) MaterialTheme.colorScheme.primary
                    else MaterialTheme.colorScheme.surfaceVariant
                )
                .padding(horizontal = 14.dp, vertical = 10.dp),
        ) {
            Row(verticalAlignment = Alignment.Bottom) {
                Text(
                    text,
                    color = if (isUser) MaterialTheme.colorScheme.onPrimary else MaterialTheme.colorScheme.onSurfaceVariant,
                    style = MaterialTheme.typography.bodyLarge,
                )
                if (streaming) {
                    Spacer(Modifier.width(2.dp))
                    BlinkingCursor(color = MaterialTheme.colorScheme.onSurfaceVariant)
                }
            }
        }
    }
}

@Composable
private fun BlinkingCursor(color: Color) {
    val transition = rememberInfiniteTransition(label = "cursor")
    val alpha by transition.animateFloat(
        initialValue = 1f,
        targetValue = 0.15f,
        animationSpec = infiniteRepeatable(
            animation = tween(600, easing = LinearEasing),
            repeatMode = RepeatMode.Reverse,
        ),
        label = "cursor-alpha",
    )
    Box(
        Modifier
            .size(width = 2.dp, height = 16.dp)
            .background(color.copy(alpha = alpha))
    )
}

/** Three softly pulsing dots, shown while waiting for the first streamed token. */
@Composable
private fun TypingIndicator() {
    Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.Start) {
        Row(
            modifier = Modifier
                .clip(RoundedCornerShape(topStart = 16.dp, topEnd = 16.dp, bottomStart = 4.dp, bottomEnd = 16.dp))
                .background(MaterialTheme.colorScheme.surfaceVariant)
                .padding(horizontal = 16.dp, vertical = 14.dp),
            horizontalArrangement = Arrangement.spacedBy(4.dp),
        ) {
            val transition = rememberInfiniteTransition(label = "typing")
            repeat(3) { index ->
                val alpha by transition.animateFloat(
                    initialValue = 0.25f,
                    targetValue = 1f,
                    animationSpec = infiniteRepeatable(
                        animation = tween(900, easing = RunyxEasing, delayMillis = index * 150),
                        repeatMode = RepeatMode.Reverse,
                    ),
                    label = "dot-$index",
                )
                Box(
                    Modifier
                        .size(7.dp)
                        .clip(CircleShape)
                        .background(MaterialTheme.colorScheme.onSurfaceVariant.copy(alpha = alpha))
                )
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
