package app.teltun

import android.content.Intent
import android.net.VpnService
import app.teltun.mobile.Mobile

/**
 * The VPN. On a Chromebook, an Android VPN carries the traffic of the whole
 * system (Chrome included); every connection goes to the phone over USB.
 */
class TunService : VpnService() {

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        if (intent?.action == ACTION_STOP) {
            Tel.disconnect()
            stopSelf()
            return START_NOT_STICKY
        }
        val pfd = Builder()
            .setSession("TelTun")
            .addAddress("172.19.0.1", 30)
            .addRoute("0.0.0.0", 0)
            .addDnsServer("1.1.1.1")
            .addDnsServer("8.8.8.8")
            .setMtu(1500)
            .addDisallowedApplication(packageName)
            .establish()
        if (pfd == null) {
            Tel.fail("errTun", "VPN permission was not given")
            stopSelf()
            return START_NOT_STICKY
        }
        Thread {
            try {
                Mobile.startTun(pfd.detachFd().toLong(), Tel.blockQuic)
            } catch (e: Exception) {
                Tel.fail("errTun", e.message ?: "")
                Tel.disconnect()
                stopSelf()
            }
        }.start()
        Tel.service = this
        return START_STICKY
    }

    override fun onRevoke() {
        // Another VPN took over, or the user turned TelTun off in settings.
        Tel.disconnect()
        stopSelf()
    }

    override fun onDestroy() {
        if (Tel.service === this) Tel.service = null
        super.onDestroy()
    }

    companion object {
        const val ACTION_STOP = "app.teltun.STOP"
    }
}
