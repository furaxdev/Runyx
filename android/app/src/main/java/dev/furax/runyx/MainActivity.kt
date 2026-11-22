package dev.furax.runyx

import android.content.Intent
import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.activity.viewModels
import androidx.compose.animation.AnimatedContent
import androidx.compose.animation.core.tween
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
import androidx.compose.animation.togetherWith
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.material3.Surface
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier

/**
 * Entry point. Routes between pairing (item 26) and chat (items 25, 27)
 * based on whether a PC is already paired, and handles incoming Share
 * Sheet intents (item 27: "partage vers Runyx") by pre-filling the chat
 * input with the shared text.
 */
class MainActivity : ComponentActivity() {
    private val viewModel: RunyxViewModel by viewModels()
    private var sharedText: String? by mutableStateOf(null)

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        viewModel.init(PairingStore(applicationContext))
        handleIncomingIntent(intent)

        setContent {
            RunyxTheme {
                Surface(modifier = Modifier.fillMaxSize()) {
                    RunyxApp(viewModel, sharedText)
                }
            }
        }
    }

    override fun onNewIntent(intent: Intent) {
        super.onNewIntent(intent)
        handleIncomingIntent(intent)
    }

    private fun handleIncomingIntent(intent: Intent?) {
        if (intent?.action == Intent.ACTION_SEND && intent.type == "text/plain") {
            sharedText = intent.getStringExtra(Intent.EXTRA_TEXT)
        }
    }
}

@Composable
fun RunyxApp(viewModel: RunyxViewModel, sharedText: String?) {
    val paired = viewModel.paired
    // Crossfade between pairing and chat instead of an abrupt cut, using
    // the same fluid easing as the web app's transitions.
    AnimatedContent(
        targetState = paired != null,
        transitionSpec = {
            fadeIn(tween(240, easing = RunyxEasing)) togetherWith fadeOut(tween(160, easing = RunyxEasing))
        },
        label = "pairing-to-chat",
    ) { isPaired ->
        if (!isPaired) {
            PairingScreen(viewModel)
        } else {
            ChatScreen(viewModel, sharedText)
        }
    }
}
