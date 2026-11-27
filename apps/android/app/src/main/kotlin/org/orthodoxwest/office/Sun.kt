package org.orthodoxwest.office

import android.Manifest
import android.annotation.SuppressLint
import android.content.Context
import android.content.pm.PackageManager
import android.location.Location
import android.location.LocationListener
import android.location.LocationManager
import android.os.Build
import android.os.Looper
import androidx.core.content.ContextCompat
import java.time.Instant

/**
 * Where the phone is, for reminders that follow the sun: read coarsely, only while the app is
 * open, and kept on the phone. The platform's own location service, with no Play Services.
 */
object PhoneLocation {
    fun allowed(context: Context): Boolean =
        ContextCompat.checkSelfPermission(context, Manifest.permission.ACCESS_COARSE_LOCATION) == PackageManager.PERMISSION_GRANTED

    /**
     * Reads the phone's place once and hands it to `done` on the main thread; nothing comes when
     * the permission is missing or no provider knows. The network's place is the coarse one; the
     * passive provider offers whatever another app last read.
     */
    @SuppressLint("MissingPermission")
    fun read(context: Context, done: (StoredPlace) -> Unit) {
        if (!allowed(context)) return
        val manager = context.getSystemService(LocationManager::class.java) ?: return
        val provider = listOf(LocationManager.NETWORK_PROVIDER, LocationManager.PASSIVE_PROVIDER)
            .firstOrNull { runCatching { manager.isProviderEnabled(it) }.getOrDefault(false) } ?: return
        val give = { l: Location? -> if (l != null) done(StoredPlace(l.latitude, l.longitude, Instant.now())) }
        try {
            if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.R) {
                manager.getCurrentLocation(provider, null, ContextCompat.getMainExecutor(context)) { l ->
                    give(l ?: manager.getLastKnownLocation(provider))
                }
            } else {
                val last = manager.getLastKnownLocation(provider)
                if (last != null) return give(last)
                @Suppress("DEPRECATION")
                manager.requestSingleUpdate(provider, LocationListener { give(it) }, Looper.getMainLooper())
            }
        } catch (_: SecurityException) {
            // The permission was taken back between the check and the call.
        } catch (_: IllegalArgumentException) {
            // The provider went away.
        }
    }
}
