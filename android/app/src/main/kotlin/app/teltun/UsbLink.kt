package app.teltun

import android.hardware.usb.UsbConstants
import android.hardware.usb.UsbDevice
import android.hardware.usb.UsbDeviceConnection
import android.hardware.usb.UsbEndpoint
import android.hardware.usb.UsbInterface
import android.hardware.usb.UsbManager
import app.teltun.mobile.USB
import java.io.IOException

/**
 * The phone's adb interface, as raw USB bulk endpoints. The Go engine speaks
 * the adb protocol over it, so no adb program is needed on this device.
 */
class UsbLink private constructor(
    private val conn: UsbDeviceConnection,
    private val intf: UsbInterface,
    private val epIn: UsbEndpoint,
    private val epOut: UsbEndpoint,
) : USB {

    override fun bulkOut(data: ByteArray): Long {
        var off = 0
        while (off < data.size) {
            val n = conn.bulkTransfer(epOut, data, off, data.size - off, 10_000)
            if (n < 0) throw IOException("USB write failed")
            off += n
        }
        // adbd needs a zero-length packet after a write that fills whole packets.
        if (data.isNotEmpty() && data.size % epOut.maxPacketSize == 0) {
            conn.bulkTransfer(epOut, ByteArray(0), 0, 10_000)
        }
        return data.size.toLong()
    }

    override fun bulkIn(max: Long): ByteArray {
        val buf = ByteArray(max.toInt())
        val n = conn.bulkTransfer(epIn, buf, buf.size, 0) // 0 = wait until data
        if (n < 0) throw IOException("USB read failed")
        return if (n == buf.size) buf else buf.copyOf(n)
    }

    fun close() {
        runCatching { conn.releaseInterface(intf) }
        runCatching { conn.close() }
    }

    companion object {
        /** The adb interface of a device: vendor class, subclass 66, protocol 1. */
        fun adbInterface(dev: UsbDevice): UsbInterface? =
            (0 until dev.interfaceCount).map { dev.getInterface(it) }.firstOrNull {
                it.interfaceClass == UsbConstants.USB_CLASS_VENDOR_SPEC &&
                    it.interfaceSubclass == 66 && it.interfaceProtocol == 1
            }

        fun findPhone(usb: UsbManager): UsbDevice? =
            usb.deviceList.values.firstOrNull { adbInterface(it) != null }

        fun open(usb: UsbManager, dev: UsbDevice): UsbLink {
            val intf = adbInterface(dev) ?: throw IOException("no adb interface")
            val conn = usb.openDevice(dev) ?: throw IOException("cannot open USB device")
            if (!conn.claimInterface(intf, true)) {
                conn.close()
                throw IOException("USB interface is busy")
            }
            var epIn: UsbEndpoint? = null
            var epOut: UsbEndpoint? = null
            for (i in 0 until intf.endpointCount) {
                val ep = intf.getEndpoint(i)
                if (ep.type != UsbConstants.USB_ENDPOINT_XFER_BULK) continue
                if (ep.direction == UsbConstants.USB_DIR_IN) epIn = ep else epOut = ep
            }
            if (epIn == null || epOut == null) {
                conn.close()
                throw IOException("adb endpoints not found")
            }
            return UsbLink(conn, intf, epIn, epOut)
        }
    }
}
