package org.orthodoxwest.office

import androidx.compose.animation.core.Animatable
import androidx.compose.animation.core.FastOutSlowInEasing
import androidx.compose.animation.core.tween
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.drawWithContent
import androidx.compose.ui.graphics.ImageBitmap
import androidx.compose.ui.graphics.layer.GraphicsLayer
import androidx.compose.ui.graphics.layer.drawLayer
import androidx.compose.ui.graphics.rememberGraphicsLayer
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Job
import kotlinx.coroutines.launch

/**
 * Lets the room dim rather than snap when the reader changes how it looks, as the web's theme
 * does: the screen as it was is held over the new one and fades away, the wall and its colours
 * with it. The system's animation scale governs it, so Remove animations changes at once.
 */
class Dissolve internal constructor(internal val layer: GraphicsLayer, private val scope: CoroutineScope) {
    internal var still: ImageBitmap? by mutableStateOf(null)
    internal val alpha = Animatable(0f)
    private var running: Job? = null

    /** Applies `change` under the screen as it is now, then lets that fade. */
    fun change(change: () -> Unit) {
        running?.cancel()
        running = scope.launch {
            try {
                still = runCatching { layer.toImageBitmap() }.getOrNull()
                alpha.snapTo(1f)
                change()
                alpha.animateTo(0f, tween(320, easing = FastOutSlowInEasing))
            } finally {
                still = null
            }
        }
    }
}

@Composable
fun rememberDissolve(): Dissolve {
    val layer = rememberGraphicsLayer()
    val scope = rememberCoroutineScope()
    return remember(layer) { Dissolve(layer, scope) }
}

/** Draws the content through `dissolve`, keeping each frame to hold over the next change. */
fun Modifier.dissolving(dissolve: Dissolve): Modifier = this.drawWithContent {
    dissolve.layer.record { this@drawWithContent.drawContent() }
    drawLayer(dissolve.layer)
    dissolve.still?.let { drawImage(it, alpha = dissolve.alpha.value) }
}
