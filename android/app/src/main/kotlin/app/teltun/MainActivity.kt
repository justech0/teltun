package app.teltun

import android.app.Activity
import android.app.PendingIntent
import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import android.content.IntentFilter
import android.graphics.Color
import android.graphics.Typeface
import android.graphics.drawable.GradientDrawable
import android.hardware.usb.UsbDevice
import android.hardware.usb.UsbManager
import android.net.VpnService
import android.os.Build
import android.os.Bundle
import android.view.Gravity
import android.view.View
import android.widget.Button
import android.widget.CheckBox
import android.widget.LinearLayout
import android.widget.ScrollView
import android.widget.TextView
import app.teltun.mobile.Mobile
import java.util.Locale

/** First test screen: connect button, status, speed and log. */
class MainActivity : Activity() {

    private lateinit var usb: UsbManager
    private lateinit var status: TextView
    private lateinit var sub: TextView
    private lateinit var button: Button
    private lateinit var speed: TextView
    private lateinit var logView: TextView

    private val tr = Locale.getDefault().language == "tr"
    private fun t(en: String, trText: String) = if (tr) trText else en

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        usb = getSystemService(USB_SERVICE) as UsbManager
        Mobile.init(filesDir.path, Tel)
        setContentView(buildUi())
        Tel.onChange = { render() }
        val filter = IntentFilter(ACTION_USB_PERMISSION)
        if (Build.VERSION.SDK_INT >= 33) registerReceiver(permReceiver, filter, RECEIVER_NOT_EXPORTED)
        else registerReceiver(permReceiver, filter)
        render()
    }

    override fun onDestroy() {
        unregisterReceiver(permReceiver)
        Tel.onChange = null
        super.onDestroy()
    }

    // ---------- UI ----------

    private fun dp(v: Int) = (v * resources.displayMetrics.density).toInt()

    private fun buildUi(): View {
        val root = LinearLayout(this).apply {
            orientation = LinearLayout.VERTICAL
            setPadding(dp(24), dp(28), dp(24), dp(16))
            background = GradientDrawable(
                GradientDrawable.Orientation.TL_BR,
                intArrayOf(Color.parseColor("#0f1a3d"), Color.parseColor("#0a0c22"), Color.parseColor("#2a1052")),
            )
        }
        root.addView(TextView(this).apply {
            text = "TelTun"
            textSize = 26f
            setTypeface(typeface, Typeface.BOLD)
            setTextColor(Color.WHITE)
        })
        root.addView(TextView(this).apply {
            text = "Made by Enes"
            textSize = 12f
            setTextColor(Color.parseColor("#8890b5"))
        })
        status = TextView(this).apply {
            textSize = 22f
            setTypeface(typeface, Typeface.BOLD)
            setTextColor(Color.WHITE)
            setPadding(0, dp(28), 0, dp(4))
        }
        root.addView(status)
        sub = TextView(this).apply {
            textSize = 14f
            setTextColor(Color.parseColor("#b7bde0"))
        }
        root.addView(sub)
        button = Button(this).apply {
            textSize = 16f
            setTextColor(Color.WHITE)
            isAllCaps = false
            background = GradientDrawable(
                GradientDrawable.Orientation.LEFT_RIGHT,
                intArrayOf(Color.parseColor("#22d3ee"), Color.parseColor("#6366f1"), Color.parseColor("#a855f7")),
            ).apply { cornerRadius = dp(14).toFloat() }
            setOnClickListener { onButton() }
        }
        root.addView(button, LinearLayout.LayoutParams(LinearLayout.LayoutParams.MATCH_PARENT, dp(56)).apply {
            topMargin = dp(20)
        })
        root.addView(CheckBox(this).apply {
            text = t("Block QUIC (faster for most sites)", "QUIC engelle (çoğu sitede daha hızlı)")
            setTextColor(Color.parseColor("#b7bde0"))
            isChecked = Tel.blockQuic
            setOnCheckedChangeListener { _, c -> Tel.blockQuic = c }
        }, LinearLayout.LayoutParams(LinearLayout.LayoutParams.WRAP_CONTENT, LinearLayout.LayoutParams.WRAP_CONTENT).apply {
            topMargin = dp(8)
        })
        speed = TextView(this).apply {
            textSize = 18f
            setTextColor(Color.WHITE)
            setPadding(0, dp(18), 0, dp(10))
        }
        root.addView(speed)
        logView = TextView(this).apply {
            textSize = 11f
            typeface = Typeface.MONOSPACE
            setTextColor(Color.parseColor("#9aa3cc"))
            setTextIsSelectable(true)
        }
        root.addView(ScrollView(this).apply { addView(logView) },
            LinearLayout.LayoutParams(LinearLayout.LayoutParams.MATCH_PARENT, 0, 1f))
        return root
    }

    private val errText: Map<String, Pair<String, String>> = mapOf(
        "errNoDevice" to ("No phone found. Plug it in and turn on USB debugging." to
            "Telefon bulunamadı. Tak ve USB hata ayıklamayı aç."),
        "errUsbPermission" to ("USB access was not allowed." to "USB erişimine izin verilmedi."),
        "errUsb" to ("Could not open the phone over USB." to "Telefon USB üzerinden açılamadı."),
        "errUnauthorized" to ("The phone did not allow this device. Tap \"Allow\" on the phone." to
            "Telefon bu cihaza izin vermedi. Telefonda \"İzin ver\"e dokun."),
        "errPush" to ("Could not send the helper to the phone." to "Yardımcı telefona gönderilemedi."),
        "errPhone" to ("The helper did not start on the phone." to "Yardımcı telefonda başlamadı."),
        "errLink" to ("Could not open the USB link." to "USB hattı açılamadı."),
        "errTest" to ("The phone has no internet. Check mobile data." to "Telefonda internet yok. Mobil veriyi kontrol et."),
        "errTun" to ("Could not start the VPN." to "VPN başlatılamadı."),
        "lost" to ("The link was lost." to "Bağlantı koptu."),
    )

    private val stepText: Map<String, Pair<String, String>> = mapOf(
        "stepDevice" to ("Talking to the phone" to "Telefonla konuşuluyor"),
        "stepPush" to ("Sending helper to the phone" to "Yardımcı telefona gönderiliyor"),
        "stepPhone" to ("Starting helper on the phone" to "Telefonda yardımcı başlıyor"),
        "stepLink" to ("Opening the USB link" to "USB hattı açılıyor"),
        "stepTest" to ("Testing the internet" to "İnternet deneniyor"),
    )

    private fun mbps(bps: Double) = String.format(Locale.US, "%.1f", bps * 8 / 1e6)

    private fun render() {
        when (Tel.state) {
            "connected" -> {
                status.text = t("Connected", "Bağlı")
                sub.text = Tel.detail.ifEmpty { t("USB tunnel is open.", "USB tüneli açık.") }
                button.text = t("Disconnect", "Bağlantıyı kes")
                button.isEnabled = true
                speed.text = "↓ ${mbps(Tel.down)} Mbps    ↑ ${mbps(Tel.up)} Mbps    · ${Tel.streams}"
            }
            "connecting", "linked" -> {
                status.text = t("Connecting…", "Bağlanıyor…")
                val s = stepText[Tel.step]
                sub.text = when {
                    Tel.authPrompt -> t("Unlock the phone and tap \"Allow USB debugging\".",
                        "Telefonun kilidini aç ve \"USB hata ayıklamaya izin ver\"e dokun.")
                    s != null -> t(s.first, s.second)
                    Tel.state == "linked" -> t("Starting the VPN", "VPN başlatılıyor")
                    else -> ""
                }
                button.text = t("Please wait", "Bekle")
                button.isEnabled = false
                speed.text = ""
            }
            else -> {
                status.text = t("Not connected", "Bağlı değil")
                val e = errText[Tel.err]
                sub.text = when {
                    e != null -> t(e.first, e.second) + if (Tel.detail.isNotEmpty()) "\n${Tel.detail}" else ""
                    else -> t("Plug in your phone with USB and press Connect.", "Telefonu USB ile tak ve Bağlan'a bas.")
                }
                button.text = t("Connect", "Bağlan")
                button.isEnabled = true
                speed.text = ""
            }
        }
        synchronized(Tel.log) { logView.text = Tel.log.joinToString("\n") }
    }

    // ---------- connect flow ----------

    private fun onButton() {
        if (Tel.state == "connected") {
            startService(Intent(this, TunService::class.java).setAction(TunService.ACTION_STOP))
            return
        }
        val dev = UsbLink.findPhone(usb)
        if (dev == null) {
            Tel.fail("errNoDevice", "")
            return
        }
        if (usb.hasPermission(dev)) openAndConnect(dev) else askPermission(dev)
    }

    private fun askPermission(dev: UsbDevice) {
        val flags = if (Build.VERSION.SDK_INT >= 31) PendingIntent.FLAG_MUTABLE else 0
        val pi = PendingIntent.getBroadcast(this, 0, Intent(ACTION_USB_PERMISSION).setPackage(packageName), flags)
        usb.requestPermission(dev, pi)
    }

    private val permReceiver = object : BroadcastReceiver() {
        override fun onReceive(c: Context, i: Intent) {
            val dev = if (Build.VERSION.SDK_INT >= 33) i.getParcelableExtra(UsbManager.EXTRA_DEVICE, UsbDevice::class.java)
            else @Suppress("DEPRECATION") i.getParcelableExtra(UsbManager.EXTRA_DEVICE)
            if (i.getBooleanExtra(UsbManager.EXTRA_PERMISSION_GRANTED, false) && dev != null) openAndConnect(dev)
            else Tel.fail("errUsbPermission", "")
        }
    }

    private fun openAndConnect(dev: UsbDevice) {
        val link = try {
            UsbLink.open(usb, dev)
        } catch (e: Exception) {
            Tel.fail("errUsb", e.message ?: "")
            return
        }
        Tel.link = link
        Tel.onStatus("connecting", "stepDevice", "", "")
        Thread {
            try {
                Mobile.connect(link)
                runOnUiThread { startVpn() }
            } catch (_: Exception) {
                // the engine already reported the error
            }
        }.start()
    }

    private fun startVpn() {
        val ask = VpnService.prepare(this)
        if (ask != null) startActivityForResult(ask, REQ_VPN) else onActivityResult(REQ_VPN, RESULT_OK, null)
    }

    @Deprecated("Activity result API is fine for this single request")
    override fun onActivityResult(requestCode: Int, resultCode: Int, data: Intent?) {
        if (requestCode != REQ_VPN) return
        if (resultCode == RESULT_OK) startService(Intent(this, TunService::class.java))
        else {
            Mobile.stop()
            Tel.fail("errTun", t("VPN permission was not given", "VPN izni verilmedi"))
        }
    }

    companion object {
        private const val ACTION_USB_PERMISSION = "app.teltun.USB_PERMISSION"
        private const val REQ_VPN = 7
    }
}
