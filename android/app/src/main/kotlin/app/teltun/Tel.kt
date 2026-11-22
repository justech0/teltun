package app.teltun

import android.os.Handler
import android.os.Looper
import app.teltun.mobile.Listener
import app.teltun.mobile.Mobile

/** App-wide tunnel state shared by the screen and the VPN service. */
object Tel : Listener {
    @Volatile var state = "idle"
    @Volatile var step = ""
    @Volatile var err = ""
    @Volatile var detail = ""
    @Volatile var down = 0.0
    @Volatile var up = 0.0
    @Volatile var totalDown = 0L
    @Volatile var totalUp = 0L
    @Volatile var streams = 0L
    @Volatile var authPrompt = false
    @Volatile var blockQuic = true
    @Volatile var service: TunService? = null
    @Volatile var link: UsbLink? = null
    val log = ArrayDeque<String>()

    /** Called on the main thread whenever something changes. */
    var onChange: (() -> Unit)? = null
    private val main = Handler(Looper.getMainLooper())
    private fun changed() = main.post { onChange?.invoke() }

    override fun onStatus(state: String, step: String, err: String, detail: String) {
        this.state = state
        this.step = step
        if (err.isNotEmpty() || state != "idle") this.err = err
        if (detail.isNotEmpty() || state == "idle") this.detail = detail
        if (state != "connecting") authPrompt = false
        if (state == "idle") {
            down = 0.0; up = 0.0
            cleanup()
        }
        changed()
    }

    override fun onStats(down: Double, up: Double, totalDown: Long, totalUp: Long, streams: Long) {
        this.down = down; this.up = up
        this.totalDown = totalDown; this.totalUp = totalUp; this.streams = streams
        changed()
    }

    override fun onLog(line: String) {
        synchronized(log) {
            log.addLast(line)
            while (log.size > 300) log.removeFirst()
        }
        changed()
    }

    override fun onAuthPrompt() {
        authPrompt = true
        changed()
    }

    fun fail(key: String, detail: String) = onStatus("idle", "", key, detail)

    /** Stop the tunnel. Going idle gives the USB device back (cleanup). */
    fun disconnect() {
        Thread {
            Mobile.stop()
            onStatus("idle", "", "", "")
        }.start()
    }

    /** Close the USB device and the VPN; safe to call more than once. */
    private fun cleanup() {
        link?.close()
        link = null
        service?.let { main.post { it.stopSelf() } }
    }
}
