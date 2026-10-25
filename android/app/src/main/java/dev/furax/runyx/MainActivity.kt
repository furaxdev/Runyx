package dev.furax.runyx

import android.content.Intent
import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.activity.viewModels
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp
import androidx.compose.foundation.layout.fillMaxSize

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
            MaterialTheme {
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
    if (paired == null) {
        PairingScreen(viewModel)
    } else {
        ChatScreen(viewModel, sharedText)
    }
}
