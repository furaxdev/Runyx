package dev.furax.runyx

import android.content.Context
import android.media.MediaRecorder
import android.os.Build
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import java.io.File
import java.net.HttpURLConnection
import java.net.URL
import java.util.UUID

/**
 * Records a push-to-talk clip (item 25) to a local file, and uploads it
 * to the paired PC's `/api/runyx/voice/transcribe` endpoint.
 */
class VoiceRecorder(private val context: Context) {
    private var recorder: MediaRecorder? = null
    private var outputFile: File? = null

    fun start() {
        val file = File.createTempFile("runyx_rec_", ".m4a", context.cacheDir)
        outputFile = file
        val r = if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.S) {
            MediaRecorder(context)
        } else {
            @Suppress("DEPRECATION")
            MediaRecorder()
        }
        r.setAudioSource(MediaRecorder.AudioSource.MIC)
        r.setOutputFormat(MediaRecorder.OutputFormat.MPEG_4)
        r.setAudioEncoder(MediaRecorder.AudioEncoder.AAC)
        r.setOutputFile(file.absolutePath)
        r.prepare()
        r.start()
        recorder = r
    }

    /** Stops recording and uploads the clip, returning the transcribed text. */
    suspend fun stopAndTranscribe(host: PairedHost): String = withContext(Dispatchers.IO) {
        recorder?.apply { stop(); release() }
        recorder = null
        val file = outputFile ?: throw IllegalStateException("no recording in progress")

        val boundary = "RunyxBoundary${UUID.randomUUID()}"
        val conn = URL("${host.baseUrl}/api/runyx/voice/transcribe").openConnection() as HttpURLConnection
        conn.requestMethod = "POST"
        conn.doOutput = true
        conn.setRequestProperty("Content-Type", "multipart/form-data; boundary=$boundary")
        if (host.token.isNotEmpty()) conn.setRequestProperty("Authorization", "Bearer ${host.token}")

        conn.outputStream.use { out ->
            out.write("--$boundary\r\n".toByteArray())
            out.write("Content-Disposition: form-data; name=\"file\"; filename=\"recording.m4a\"\r\n".toByteArray())
            out.write("Content-Type: audio/mp4\r\n\r\n".toByteArray())
            out.write(file.readBytes())
            out.write("\r\n--$boundary--\r\n".toByteArray())
        }

        val responseText = conn.inputStream.bufferedReader().readText()
        file.delete()
        org.json.JSONObject(responseText).optString("text", "")
    }

    fun cancel() {
        runCatching { recorder?.apply { stop(); release() } }
        recorder = null
        outputFile?.delete()
    }
}
