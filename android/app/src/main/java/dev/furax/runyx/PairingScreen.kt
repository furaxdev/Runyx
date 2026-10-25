package dev.furax.runyx

import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.foundation.layout.*
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp

/**
 * Pairing screen (item 26): scan the PC's QR code (via a photo, see
 * Pairing.kt) or paste the `runyx://pair?...` link directly (e.g. shared
 * from a QR-reading app, or typed for same-machine testing).
 */
@Composable
fun PairingScreen(viewModel: RunyxViewModel) {
    var manualCode by remember { mutableStateOf("") }

    val takePicture = rememberLauncherForActivityResult(ActivityResultContracts.TakePicturePreview()) { bitmap ->
        if (bitmap != null) {
            val decoded = decodeQRCode(bitmap)
            if (decoded != null) {
                viewModel.pairWithCode(decoded)
            } else {
                viewModel.pairingError = "No QR code found in the photo — try again with better lighting/focus."
            }
        }
    }

    Column(
        modifier = Modifier.fillMaxSize().padding(24.dp),
        verticalArrangement = Arrangement.Center,
        horizontalAlignment = Alignment.CenterHorizontally
    ) {
        Text("Runyx", style = MaterialTheme.typography.displaySmall)
        Spacer(Modifier.height(8.dp))
        Text("Pair this phone with your Runyx PC.")
        Spacer(Modifier.height(24.dp))

        Button(onClick = { takePicture.launch(null) }) {
            Text("Scan QR code")
        }

        Spacer(Modifier.height(16.dp))
        Text("— or —")
        Spacer(Modifier.height(16.dp))

        OutlinedTextField(
            value = manualCode,
            onValueChange = { manualCode = it },
            label = { Text("runyx://pair?d=...") },
            modifier = Modifier.fillMaxWidth()
        )
        Spacer(Modifier.height(8.dp))
        Button(onClick = { viewModel.pairWithCode(manualCode) }, modifier = Modifier.fillMaxWidth()) {
            Text("Pair")
        }

        viewModel.pairingError?.let {
            Spacer(Modifier.height(16.dp))
            Text(it, color = MaterialTheme.colorScheme.error)
        }
    }
}
