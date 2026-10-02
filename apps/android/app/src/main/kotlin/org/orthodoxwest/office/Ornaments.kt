package org.orthodoxwest.office

import androidx.compose.foundation.Canvas
import androidx.compose.foundation.Image
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.foundation.background
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.graphics.BlendMode
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.CompositingStrategy
import androidx.compose.ui.graphics.Path
import androidx.compose.ui.graphics.PathFillType
import androidx.compose.ui.graphics.drawscope.DrawScope
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.graphics.drawscope.clipRect
import androidx.compose.ui.graphics.drawscope.scale
import androidx.compose.ui.graphics.drawscope.translate
import androidx.compose.ui.graphics.graphicsLayer
import androidx.compose.ui.graphics.vector.PathParser
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

/** An SVG path, as the web's templates and ornaments draw it. */
private fun svg(d: String): Path = PathParser().parsePathString(d).toPath()

// Headpiece rule (macros.html `headpiece`), viewBox 56×20.
private val SprigRule = svg("M2 10h52")

// The painted ornaments (static/ornaments/, drawn by tools/genornaments.py): shapes only, each
// filled with its colour token. The consecration cross in its compass circle, viewBox 40×40; the
// ring is an even-odd pair of circles.
private val ConsecrationRing = svg("M0.70,20.00a19.30,19.30 0 1,0 38.60,0a19.30,19.30 0 1,0 -38.60,0ZM1.90,20.00a18.10,18.10 0 1,0 36.20,0a18.10,18.10 0 1,0 -36.20,0Z").apply { fillType = PathFillType.EvenOdd }
private val ConsecrationArms = svg(
    "M18.75,18.75 18.74,17.65 18.72,16.56 18.67,15.46 18.59,14.36 18.49,13.27 18.36,12.17 18.21,11.07 18.02,9.98 17.81,8.88 17.57,7.79 17.29,6.69 16.98,5.59 16.64,4.50 16.27,3.40 16.79,3.71 17.32,3.62 17.85,3.54 18.39,3.48 18.92,3.43 19.46,3.41 20.00,3.40 20.54,3.41 21.08,3.43 21.61,3.48 22.15,3.54 22.68,3.62 23.21,3.71 23.73,3.40 23.36,4.50 23.02,5.59 22.71,6.69 22.43,7.79 22.19,8.88 21.98,9.98 21.79,11.07 21.64,12.17 21.51,13.27 21.41,14.36 21.33,15.46 21.28,16.56 21.26,17.65 21.25,18.75Z" +
        "M21.25,18.75 22.35,18.74 23.44,18.72 24.54,18.67 25.64,18.59 26.73,18.49 27.83,18.36 28.93,18.21 30.02,18.02 31.12,17.81 32.21,17.57 33.31,17.29 34.41,16.98 35.50,16.64 36.60,16.27 36.29,16.79 36.38,17.32 36.46,17.85 36.52,18.39 36.57,18.92 36.59,19.46 36.60,20.00 36.59,20.54 36.57,21.08 36.52,21.61 36.46,22.15 36.38,22.68 36.29,23.21 36.60,23.73 35.50,23.36 34.41,23.02 33.31,22.71 32.21,22.43 31.12,22.19 30.02,21.98 28.93,21.79 27.83,21.64 26.73,21.51 25.64,21.41 24.54,21.33 23.44,21.28 22.35,21.26 21.25,21.25Z" +
        "M21.25,21.25 21.26,22.35 21.28,23.44 21.33,24.54 21.41,25.64 21.51,26.73 21.64,27.83 21.79,28.93 21.98,30.02 22.19,31.12 22.43,32.21 22.71,33.31 23.02,34.41 23.36,35.50 23.73,36.60 23.21,36.29 22.68,36.38 22.15,36.46 21.61,36.52 21.08,36.57 20.54,36.59 20.00,36.60 19.46,36.59 18.92,36.57 18.39,36.52 17.85,36.46 17.32,36.38 16.79,36.29 16.27,36.60 16.64,35.50 16.98,34.41 17.29,33.31 17.57,32.21 17.81,31.12 18.02,30.02 18.21,28.93 18.36,27.83 18.49,26.73 18.59,25.64 18.67,24.54 18.72,23.44 18.74,22.35 18.75,21.25Z" +
        "M18.75,21.25 17.65,21.26 16.56,21.28 15.46,21.33 14.36,21.41 13.27,21.51 12.17,21.64 11.07,21.79 9.98,21.98 8.88,22.19 7.79,22.43 6.69,22.71 5.59,23.02 4.50,23.36 3.40,23.73 3.71,23.21 3.62,22.68 3.54,22.15 3.48,21.61 3.43,21.08 3.41,20.54 3.40,20.00 3.41,19.46 3.43,18.92 3.48,18.39 3.54,17.85 3.62,17.32 3.71,16.79 3.40,16.27 4.50,16.64 5.59,16.98 6.69,17.29 7.79,17.57 8.88,17.81 9.98,18.02 11.07,18.21 12.17,18.36 13.27,18.49 14.36,18.59 15.46,18.67 16.56,18.72 17.65,18.74 18.75,18.75Z" +
        "M18.70,18.70 21.30,18.70 21.30,21.30 18.70,21.30Z",
)

// The same cross without its circle, viewBox 20×20.
private val CrossArms = svg(
    "M8.50,8.50 8.49,7.94 8.47,7.37 8.42,6.81 8.35,6.24 8.26,5.68 8.14,5.11 7.99,4.55 7.82,3.99 7.62,3.42 7.39,2.86 7.13,2.29 6.84,1.73 6.53,1.16 6.18,0.60 6.70,1.20 7.23,1.02 7.77,0.87 8.32,0.75 8.88,0.67 9.44,0.62 10.00,0.60 10.56,0.62 11.12,0.67 11.68,0.75 12.23,0.87 12.77,1.02 13.30,1.20 13.82,0.60 13.47,1.16 13.16,1.73 12.87,2.29 12.61,2.86 12.38,3.42 12.18,3.99 12.01,4.55 11.86,5.11 11.74,5.68 11.65,6.24 11.58,6.81 11.53,7.37 11.51,7.94 11.50,8.50Z" +
        "M11.50,8.50 12.06,8.49 12.63,8.47 13.19,8.42 13.76,8.35 14.32,8.26 14.89,8.14 15.45,7.99 16.01,7.82 16.58,7.62 17.14,7.39 17.71,7.13 18.27,6.84 18.84,6.53 19.40,6.18 18.80,6.70 18.98,7.23 19.13,7.77 19.25,8.32 19.33,8.88 19.38,9.44 19.40,10.00 19.38,10.56 19.33,11.12 19.25,11.68 19.13,12.23 18.98,12.77 18.80,13.30 19.40,13.82 18.84,13.47 18.27,13.16 17.71,12.87 17.14,12.61 16.58,12.38 16.01,12.18 15.45,12.01 14.89,11.86 14.32,11.74 13.76,11.65 13.19,11.58 12.63,11.53 12.06,11.51 11.50,11.50Z" +
        "M11.50,11.50 11.51,12.06 11.53,12.63 11.58,13.19 11.65,13.76 11.74,14.32 11.86,14.89 12.01,15.45 12.18,16.01 12.38,16.58 12.61,17.14 12.87,17.71 13.16,18.27 13.47,18.84 13.82,19.40 13.30,18.80 12.77,18.98 12.23,19.13 11.68,19.25 11.12,19.33 10.56,19.38 10.00,19.40 9.44,19.38 8.88,19.33 8.32,19.25 7.77,19.13 7.23,18.98 6.70,18.80 6.18,19.40 6.53,18.84 6.84,18.27 7.13,17.71 7.39,17.14 7.62,16.58 7.82,16.01 7.99,15.45 8.14,14.89 8.26,14.32 8.35,13.76 8.42,13.19 8.47,12.63 8.49,12.06 8.50,11.50Z" +
        "M8.50,11.50 7.94,11.51 7.37,11.53 6.81,11.58 6.24,11.65 5.68,11.74 5.11,11.86 4.55,12.01 3.99,12.18 3.42,12.38 2.86,12.61 2.29,12.87 1.73,13.16 1.16,13.47 0.60,13.82 1.20,13.30 1.02,12.77 0.87,12.23 0.75,11.68 0.67,11.12 0.62,10.56 0.60,10.00 0.62,9.44 0.67,8.88 0.75,8.32 0.87,7.77 1.02,7.23 1.20,6.70 0.60,6.18 1.16,6.53 1.73,6.84 2.29,7.13 2.86,7.39 3.42,7.62 3.99,7.82 4.55,7.99 5.11,8.14 5.68,8.26 6.24,8.35 6.81,8.42 7.37,8.47 7.94,8.49 8.50,8.50Z" +
        "M8.40,8.40 11.60,8.40 11.60,11.60 8.40,11.60Z",
)

// The Lauds sun and the Vespers and Compline moon, viewBox 24×24.
private val Sun = svg(
    "M17.20,12.00A5.2,5.2 0 1,0 6.80,12.00A5.2,5.2 0 1,0 17.20,12.00Z" +
        "M13.45,5.60 12.00,0.40 10.55,5.60Z" +
        "M16.46,7.18 16.70,3.86 13.94,5.73Z" +
        "M18.27,10.06 22.05,6.20 16.82,7.54Z" +
        "M18.40,13.45 21.40,12.00 18.40,10.55Z" +
        "M16.82,16.46 22.05,17.80 18.27,13.94Z" +
        "M13.94,18.27 16.70,20.14 16.46,16.82Z" +
        "M10.55,18.40 12.00,23.60 13.45,18.40Z" +
        "M7.54,16.82 7.30,20.14 10.06,18.27Z" +
        "M5.73,13.94 1.95,17.80 7.18,16.46Z" +
        "M5.60,10.55 2.60,12.00 5.60,13.45Z" +
        "M7.18,7.54 1.95,6.20 5.73,10.06Z" +
        "M10.06,5.73 7.30,3.86 7.54,7.18Z",
)
private val Moon = svg("M14.00 2.61A9.6 9.6 0 1 0 20.18 17.02A7.9 7.9 0 0 1 14.00 2.61Z")

/** Home's period glyphs (home.html), viewBox 24×16, filled: the rising sun, the sun, the moon. */
enum class Period(val path: Path) {
    MORNING(
        svg(
            "M7.60,13.20A4.4,4.4 0 0,1 16.40,13.20ZM1.5,13.40h21v1.1h-21ZM7.36,10.03L3.32,9.60L6.48,12.16ZM8.92,8.50L3.66,4.86L7.30,10.12Z" +
                "M10.96,7.68L8.40,4.52L8.83,8.56ZM13.15,7.70L12.00,1.40L10.85,7.70ZM15.17,8.56L15.60,4.52L13.04,7.68Z" +
                "M16.70,10.12L20.34,4.86L15.08,8.50ZM17.52,12.16L20.68,9.60L16.64,10.03Z",
        ),
    ),
    DAY(
        svg(
            "M15.30,8.00A3.3,3.3 0 1,0 8.70,8.00A3.3,3.3 0 1,0 15.30,8.00ZM12.95,3.50L12.00,0.30L11.05,3.50ZM15.07,4.58L15.10,2.63L13.43,3.63Z" +
                "M16.37,6.57L18.67,4.15L15.42,4.93ZM16.50,8.95L18.20,8.00L16.50,7.05ZM15.42,11.07L18.67,11.85L16.37,9.43Z" +
                "M13.43,12.37L15.10,13.37L15.07,11.42ZM11.05,12.50L12.00,15.70L12.95,12.50ZM8.93,11.42L8.90,13.37L10.57,12.37Z" +
                "M7.63,9.43L5.33,11.85L8.58,11.07ZM7.50,7.05L5.80,8.00L7.50,8.95ZM8.58,4.93L5.33,4.15L7.63,6.57ZM10.57,3.63L8.90,2.63L8.93,4.58Z",
        ),
    ),
    EVENING(svg("M13.32 1.80A6.34 6.34 0 1 0 17.40 11.32A5.21 5.21 0 0 1 13.32 1.80Z")),
}

// The ordo's abstinence fish (calendar.html `icon-fish`), viewBox 24×12.
private val Fish = svg("M1 6 C5 1.2 13 1.2 17 6 C13 10.8 5 10.8 1 6 Z M16.5 6 L23 1.5 L21.2 6 L23 10.5 Z")

// The Apse vault tile (static/ornaments/vault.svg), viewBox 528×528: parish eight-ray stars in
// three sizes, set by hand rather than on a lattice, each with its own share of the light. A star
// across the tile's edge is drawn again on the far side, so the repeat is seamless.
private val VaultStars: List<Pair<Float, Path>> = listOf(
    0.58f to "M315.25,60.42 315.36,64.37 316.73,63.55 315.74,64.84 319.39,65.32 315.77,65.45 316.60,66.85 315.29,65.89 314.78,69.68 314.66,65.84 313.42,66.54 314.33,65.34 310.30,64.86 314.32,64.77 313.60,63.54 314.74,64.33Z",
    0.53f to "M92.77,149.56 93.25,155.13 94.98,154.01 93.89,155.75 99.43,156.16 93.92,156.65 94.96,158.32 93.23,157.18 92.86,163.11 92.36,157.32 90.73,158.31 91.89,156.59 85.97,156.24 91.87,155.81 90.31,153.73 92.42,155.27Z",
    0.70f to "M257.97,230.46 258.27,234.64 259.47,233.89 258.67,235.11 262.30,235.43 258.61,235.69 259.40,236.88 258.22,236.09 257.91,240.04 257.67,236.04 256.18,237.14 257.19,235.71 253.89,235.38 257.25,235.11 256.33,233.77 257.64,234.67Z",
    0.51f to "M256.18,325.36 256.10,330.96 258.16,329.75 256.68,331.56 261.92,332.43 256.53,332.37 257.59,334.24 255.93,332.87 255.04,338.68 255.08,332.94 253.07,334.04 254.46,332.26 249.64,331.38 254.59,331.36 253.72,329.64 255.28,330.82Z",
    0.59f to "M507.09,159.21 507.53,162.53 508.79,161.60 507.99,162.92 511.80,163.07 507.91,163.50 508.87,164.74 507.57,163.92 507.43,167.39 507.02,163.91 505.69,164.97 506.58,163.58 503.09,163.44 506.56,163.01 505.74,161.86 506.91,162.51Z",
    0.60f to "M432.92,-7.99 432.91,-4.14 434.14,-4.81 433.29,-3.71 436.89,-3.13 433.29,-3.10 434.00,-1.79 432.79,-2.78 432.23,0.75 432.20,-2.75 430.92,-2.07 431.77,-3.21 428.07,-3.83 431.85,-3.83 430.96,-5.35 432.32,-4.21Z",
    0.60f to "M432.92,520.01 432.91,523.86 434.14,523.19 433.29,524.29 436.89,524.87 433.29,524.90 434.00,526.21 432.79,525.22 432.23,528.75 432.20,525.25 430.92,525.93 431.77,524.79 428.07,524.17 431.85,524.17 430.96,522.65 432.32,523.79Z",
    0.73f to "M34.67,338.50 34.62,343.55 36.68,342.43 35.18,344.24 40.36,345.20 35.05,345.10 36.08,347.02 34.41,345.69 33.40,351.43 33.52,345.60 31.65,346.57 32.92,344.93 27.11,343.89 33.16,344.09 32.00,342.06 33.73,343.44Z",
    0.58f to "M24.75,18.73 25.63,24.32 27.49,22.72 26.21,24.84 32.07,24.72 26.33,25.62 27.85,27.44 25.83,26.29 25.91,32.12 25.01,26.26 23.11,27.92 24.37,25.80 18.59,25.89 24.17,24.94 22.78,23.17 24.84,24.38Z",
    0.67f to "M307.58,-22.81 308.02,-18.98 309.28,-19.89 308.41,-18.54 312.03,-18.37 308.42,-17.97 309.47,-16.61 308.05,-17.53 307.88,-13.93 307.48,-17.56 306.20,-16.60 307.04,-17.92 303.56,-18.09 307.01,-18.50 306.18,-19.69 307.39,-18.98Z",
    0.67f to "M307.58,505.19 308.02,509.02 309.28,508.11 308.41,509.46 312.03,509.63 308.42,510.03 309.47,511.39 308.05,510.47 307.88,514.07 307.48,510.44 306.20,511.40 307.04,510.08 303.56,509.91 307.01,509.50 306.18,508.31 307.39,509.02Z",
    0.85f to "M147.61,475.74 147.92,479.70 149.36,478.70 148.26,480.18 151.84,480.44 148.35,480.75 149.26,482.08 147.91,481.15 147.62,484.69 147.34,481.11 146.01,482.06 146.98,480.71 143.09,480.45 146.85,480.13 146.10,478.94 147.31,479.71Z",
    0.84f to "M406.55,315.30 407.08,319.54 408.32,318.55 407.54,319.88 411.02,319.96 407.62,320.48 408.52,321.70 407.15,320.83 407.14,325.03 406.61,320.91 405.28,321.99 406.11,320.58 402.06,320.50 406.07,319.95 405.03,318.60 406.54,319.58Z",
    0.70f to "M99.50,56.14 99.76,59.67 101.19,58.70 100.13,60.13 104.23,60.46 100.20,60.72 100.90,61.89 99.71,61.06 99.40,64.74 99.17,61.06 97.72,62.09 98.80,60.66 95.37,60.35 98.76,60.11 97.92,58.84 99.18,59.72Z",
    0.71f to "M98.10,231.98 98.31,235.52 99.52,234.80 98.68,235.99 102.20,236.37 98.61,236.54 99.39,237.74 98.28,237.03 97.86,240.51 97.70,236.89 96.23,237.92 97.21,236.55 93.59,236.13 97.28,235.94 96.45,234.63 97.71,235.54Z",
    0.75f to "M7.20,458.55 7.50,462.46 8.83,461.51 7.97,462.82 11.76,463.10 7.99,463.44 8.75,464.64 7.53,463.84 7.25,467.95 6.92,463.89 5.77,464.60 6.56,463.41 3.03,463.15 6.48,462.82 5.59,461.51 6.94,462.43Z",
    0.75f to "M535.20,458.55 535.50,462.46 536.83,461.51 535.97,462.82 539.76,463.10 535.99,463.44 536.75,464.64 535.53,463.84 535.25,467.95 534.92,463.89 533.77,464.60 534.56,463.41 531.03,463.15 534.48,462.82 533.59,461.51 534.94,462.43Z",
    0.82f to "M425.33,223.57 425.34,227.02 426.82,226.11 425.66,227.45 429.12,227.98 425.74,228.04 426.48,229.38 425.24,228.39 424.68,231.93 424.64,228.40 423.37,229.07 424.30,227.90 420.82,227.34 424.37,227.34 423.58,226.00 424.76,226.89Z",
    0.71f to "M177.83,264.25 177.93,267.93 179.42,266.97 178.32,268.35 182.39,268.84 178.29,268.92 179.04,270.17 177.86,269.30 177.40,272.89 177.28,269.31 176.05,270.01 176.84,268.88 172.67,268.36 176.95,268.29 176.16,267.00 177.34,267.83Z",
    0.75f to "M380.90,459.22 381.53,462.96 382.62,462.05 381.96,463.29 385.62,463.21 382.06,463.85 382.92,464.94 381.69,464.32 381.74,467.96 381.11,464.34 380.02,465.20 380.62,464.00 377.03,464.03 380.56,463.39 379.49,462.11 380.99,463.00Z",
    0.57f to "M228.51,494.54 228.93,498.66 230.06,497.89 229.35,499.08 232.80,499.28 229.31,499.65 230.39,501.04 228.96,500.09 228.77,503.89 228.38,500.08 227.24,500.88 227.94,499.71 224.14,499.52 227.89,499.10 227.17,498.00 228.34,498.72Z",
    0.55f to "M384.63,144.16 384.83,149.66 386.75,148.42 385.35,150.26 391.36,150.85 385.39,151.06 386.64,153.02 384.82,151.73 384.18,156.86 383.98,151.53 381.88,152.96 383.43,150.97 378.11,150.39 383.49,150.19 382.01,148.04 384.02,149.60Z",
    0.83f to "M-9.64,237.95 -9.66,243.87 -7.82,242.77 -8.97,244.38 -3.21,245.23 -9.07,245.28 -8.13,247.02 -9.74,245.84 -10.56,251.63 -10.59,245.76 -12.58,246.93 -11.09,245.10 -16.04,244.37 -11.00,244.32 -12.16,242.41 -10.44,243.77Z",
    0.83f to "M518.36,237.95 518.34,243.87 520.18,242.77 519.03,244.38 524.79,245.23 518.93,245.28 519.87,247.02 518.26,245.84 517.44,251.63 517.41,245.76 515.42,246.93 516.91,245.10 511.96,244.37 517.00,244.32 515.84,242.41 517.56,243.77Z",
    0.63f to "M430.69,393.99 430.80,397.53 432.02,396.86 431.22,397.99 435.10,398.53 431.20,398.63 431.95,399.95 430.68,398.94 430.20,402.62 430.10,398.97 428.75,399.78 429.70,398.53 425.66,397.99 429.80,397.96 428.85,396.48 430.20,397.56Z",
    0.45f to "M442.40,79.21 442.98,83.00 444.23,82.01 443.43,83.39 446.94,83.43 443.40,83.96 444.34,85.13 443.05,84.37 443.07,88.44 442.47,84.52 441.22,85.49 441.99,84.11 438.68,84.04 442.07,83.52 440.95,82.20 442.39,83.06Z",
    0.78f to "M200.82,108.49 200.94,113.89 203.14,112.44 201.52,114.47 206.51,115.14 201.41,115.27 202.67,117.25 200.88,115.92 200.17,121.09 200.04,115.77 198.18,116.91 199.35,115.24 194.61,114.52 199.42,114.32 198.24,112.34 200.09,113.70Z",
    0.82f to "M267.82,424.94 268.13,428.78 269.45,427.87 268.50,429.21 272.01,429.47 268.61,429.80 269.43,431.07 268.11,430.15 267.85,434.13 267.52,430.25 266.22,431.11 267.08,429.80 263.17,429.50 267.13,429.19 266.16,427.82 267.53,428.76Z",
    0.56f to "M87.45,415.99 87.80,421.33 89.48,420.33 88.42,421.98 94.15,422.54 88.32,422.84 89.51,424.68 87.73,423.44 87.19,429.38 86.87,423.46 85.11,424.54 86.39,422.77 80.52,422.28 86.35,421.98 85.10,420.10 86.94,421.42Z",
    0.82f to "M179.63,369.98 180.10,376.15 181.84,374.91 180.73,376.66 186.81,376.99 180.74,377.49 182.18,379.47 180.17,378.10 179.81,383.20 179.34,378.06 177.64,379.23 178.79,377.49 173.48,377.17 178.77,376.70 177.31,374.73 179.29,376.07Z",
    0.82f to "M481.92,311.59 482.45,315.19 483.62,314.24 482.89,315.52 486.47,315.56 482.88,316.08 483.82,317.24 482.60,316.61 482.51,319.90 481.97,316.62 480.80,317.50 481.52,316.21 478.00,316.15 481.54,315.63 480.43,314.30 481.89,315.19Z",
    0.84f to "M348.72,386.94 348.99,390.53 350.22,389.76 349.39,391.00 353.09,391.34 349.40,391.60 350.30,392.96 348.93,391.94 348.61,396.13 348.35,392.02 346.90,393.01 347.97,391.56 344.39,391.24 347.93,390.97 347.21,389.80 348.37,390.56Z",
    0.61f to "M114.83,304.77 115.03,308.23 116.19,307.58 115.43,308.71 119.46,309.14 115.42,309.32 116.16,310.56 114.97,309.74 114.54,313.38 114.35,309.72 113.09,310.47 114.03,309.23 110.33,308.84 114.03,308.69 113.01,307.20 114.40,308.23Z",
    0.50f to "M280.60,142.66 280.65,148.65 282.63,147.47 281.16,149.31 287.21,150.11 281.12,150.13 282.23,152.01 280.53,150.77 279.72,156.26 279.64,150.69 277.52,151.97 279.11,150.01 274.27,149.27 279.16,149.16 277.87,147.05 279.78,148.56Z",
    0.74f to "M329.96,208.81 330.28,212.54 331.45,211.80 330.70,212.99 334.47,213.27 330.71,213.59 331.70,215.00 330.26,213.97 330.00,217.50 329.69,213.99 328.39,214.90 329.24,213.60 325.63,213.31 329.22,212.98 328.32,211.65 329.69,212.60Z",
    0.67f to "M175.81,190.27 176.14,194.32 177.45,193.35 176.59,194.66 180.31,194.90 176.54,195.23 177.42,196.46 176.19,195.68 175.95,199.55 175.61,195.66 174.21,196.70 175.25,195.24 171.70,195.03 175.19,194.69 174.37,193.50 175.57,194.25Z",
    0.83f to "M186.60,31.00 186.68,34.42 187.95,33.70 186.98,34.90 190.73,35.41 187.01,35.46 187.89,36.90 186.60,35.90 186.05,39.47 185.99,35.82 184.56,36.68 185.62,35.37 181.51,34.81 185.64,34.78 184.94,33.54 186.10,34.43Z",
    0.77f to "M331.66,289.78 331.95,293.47 333.20,292.64 332.39,293.87 335.92,294.19 332.37,294.48 333.16,295.69 331.95,294.91 331.64,298.27 331.37,294.84 329.99,295.84 330.88,294.50 327.56,294.17 330.97,293.90 330.00,292.52 331.34,293.42Z",
    0.69f to "M28.75,100.18 28.76,104.12 30.31,103.11 29.16,104.50 32.82,105.02 29.08,105.05 29.80,106.27 28.68,105.42 28.13,109.66 28.09,105.48 26.70,106.28 27.69,105.00 23.82,104.44 27.80,104.42 27.09,103.19 28.20,103.98Z",
).map { (alpha, d) -> alpha to svg(d) }

/** Draws a `vw`×`vh` viewBox fitted and centred in this scope, as SVG's default `meet`. */
private fun DrawScope.fitted(vw: Float, vh: Float, draw: DrawScope.() -> Unit) {
    val k = minOf(size.width / vw, size.height / vh)
    translate((size.width - vw * k) / 2f, (size.height - vh * k) / 2f) {
        scale(k, k, pivot = Offset.Zero) { draw() }
    }
}

@Composable
private fun Sprig() {
    val ink = LocalPalette.current.lining
    Canvas(Modifier.size(53.dp, 14.4.dp).graphicsLayer { alpha = 0.55f }) {
        fitted(56f, 20f) { drawPath(SprigRule, ink, style = Stroke(0.8f)) }
    }
}

/** The headpiece above a page's title: a painted cross between two short rules. */
@Composable
fun Headpiece(modifier: Modifier = Modifier) {
    Row(modifier, horizontalArrangement = Arrangement.spacedBy(10.dp), verticalAlignment = Alignment.CenterVertically) {
        Sprig()
        Text("✠", style = TextStyle(fontFamily = CrossFont, fontSize = 11.sp, color = LocalPalette.current.lining))
        Sprig()
    }
}

/**
 * The sign set alone into an hour title's upper rule: the sun at Lauds, sung at sunrise, and the
 * moon at Vespers and Compline, at its going down, filled as painters drew them; the cross at the
 * other hours.
 */
@Composable
fun HourSign(hour: String) {
    val p = LocalPalette.current
    when (hour) {
        "lauds" -> Canvas(Modifier.size(14.72.dp)) { fitted(24f, 24f) { drawPath(Sun, p.gold) } }
        "vespers", "compline" -> Canvas(Modifier.size(16.64.dp)) { fitted(24f, 24f) { drawPath(Moon, p.moonInk) } }
        else -> Text("✠", style = TextStyle(fontFamily = CrossFont, fontSize = 11.sp, color = p.lining))
    }
}

/**
 * A painted double rule, broken by `gap` at its centre (where a headpiece's sign sits) and
 * optionally interrupted by a lozenge in the day's colour, ringed in the lining. `heavy` is the
 * hour title's upper rule: 2dp over 1dp, as the home niche's lining.
 */
@Composable
fun DoubleRule(modifier: Modifier = Modifier, gap: Dp = 0.dp, lozenge: Color? = null, color: Color = LocalOrnament.current.line, heavy: Boolean = false) {
    val p = LocalPalette.current
    Canvas(modifier.fillMaxWidth().height(9.dp)) {
        val mid = size.height / 2f
        val half = gap.toPx() / 2f
        // Each line's centre from the middle, and its weight.
        val lines = if (heavy) listOf(-2f to 2f, 2.5f to 1f) else listOf(-1.5f to 1f, 1.5f to 1f)
        for ((at, weight) in lines) {
            val y = mid + at.dp.toPx()
            val line = weight.dp.toPx()
            if (half > 0f) {
                drawLine(color, Offset(0f, y), Offset(size.width / 2f - half, y), line)
                drawLine(color, Offset(size.width / 2f + half, y), Offset(size.width, y), line)
            } else {
                drawLine(color, Offset(0f, y), Offset(size.width, y), line)
            }
        }
        if (lozenge != null) lozenge(Offset(size.width / 2f, mid), 4.5.dp.toPx(), lozenge, p.lining)
    }
}

/** A diamond, the lorica boards' lozenge: filled, with an optional edge. */
fun DrawScope.lozenge(center: Offset, r: Float, fill: Color, edge: Color?) {
    val p = Path().apply {
        moveTo(center.x, center.y - r)
        lineTo(center.x + r, center.y)
        lineTo(center.x, center.y + r)
        lineTo(center.x - r, center.y)
        close()
    }
    drawPath(p, fill)
    if (edge != null) drawPath(p, edge, style = Stroke(0.8.dp.toPx()))
}

/** A small free-standing lozenge in the painted line's ink, as the ✦ that closes a page. */
@Composable
fun Diamond(modifier: Modifier = Modifier, size: Dp = 7.dp) {
    val o = LocalOrnament.current
    Canvas(modifier.size(size)) { lozenge(center, this.size.minDimension / 2f, o.line, null) }
}

/**
 * The consecration cross in its compass circle, as painted where the bishop anointed the walls:
 * the hour's end, home's crown and the header's mark.
 */
@Composable
fun ConsecrationCross(modifier: Modifier) {
    val ink = LocalPalette.current.lining
    Canvas(modifier) {
        fitted(40f, 40f) {
            drawPath(ConsecrationRing, ink)
            drawPath(ConsecrationArms, ink)
        }
    }
}

/** The same cross without its circle: between the office's parts, and before a first-class feast. */
@Composable
fun PaintedCross(modifier: Modifier) {
    val ink = LocalPalette.current.lining
    Canvas(modifier) { fitted(20f, 20f) { drawPath(CrossArms, ink) } }
}

@Composable
fun PeriodIcon(period: Period, color: Color, modifier: Modifier = Modifier) {
    Canvas(modifier.size(19.dp, 13.dp).graphicsLayer { alpha = 0.7f }) {
        fitted(24f, 16f) { drawPath(period.path, color) }
    }
}

@Composable
fun FishIcon(color: Color, modifier: Modifier = Modifier) {
    Canvas(modifier.size(18.dp, 9.dp)) { fitted(24f, 12f) { drawPath(Fish, color) } }
}

/** The plaster wall behind every page, baked from the web's layers (tools/bake-plaster.py). */
@Composable
fun PlasterWall() {
    Image(painterResource(LocalPalette.current.plaster), null, Modifier.fillMaxSize(), contentScale = ContentScale.Crop)
}

/**
 * The vault tile's edge for a field `width` wide (`--apse-tile`): the stars spread as the screen
 * widens, so a painted ceiling never reads as a wallpaper of small stars. The fields span the
 * screen, so their width is the web's viewport.
 */
private fun vaultTile(width: Dp): Dp = when {
    width >= 2400.dp -> 960.dp
    width >= 1800.dp -> 832.dp
    width >= WideFrom -> 704.dp
    else -> 528.dp
}

/**
 * The Apse vault (apse-vault.md): the hand-set stars of one tile, each at its own opacity, in
 * the ornament's gold, faded by stops of (fraction, alpha).
 */
fun DrawScope.vaultTiles(ink: Color, fade: List<Pair<Float, Float>>) {
    val tile = vaultTile(size.width.toDp()).toPx()
    val k = tile / 528f
    // Phase from the top centre, as the web anchors home's field: one tile centred on the page.
    val x0 = (size.width / 2f - tile / 2f).mod(tile) - tile
    var y = 0f
    while (y < size.height) {
        var x = x0
        while (x < size.width) {
            translate(x, y) {
                // Each tile clipped to itself, as a repeating mask is, so a star across its edge
                // is painted once, not by both tiles.
                clipRect(0f, 0f, tile, tile) {
                    scale(k, k, pivot = Offset.Zero) {
                        for ((alpha, star) in VaultStars) drawPath(star, ink, alpha = alpha)
                    }
                }
            }
            x += tile
        }
        y += tile
    }
    val mask = Brush.verticalGradient(*fade.map { (at, a) -> at to Color.Black.copy(alpha = a) }.toTypedArray())
    drawRect(mask, blendMode = BlendMode.DstIn)
}

/** A field of the Apse vault behind `content`, faded by stops of (fraction, alpha). */
@Composable
fun VaultField(modifier: Modifier, fade: List<Pair<Float, Float>>) {
    val palette = LocalPalette.current
    if (!palette.dark) return
    val ink = LocalOrnament.current.flat
    Canvas(modifier.graphicsLayer { compositingStrategy = CompositingStrategy.Offscreen }) { vaultTiles(ink, fade) }
}

/** A hairline across the measure. */
@Composable
fun Hairline(color: Color, modifier: Modifier = Modifier) {
    Box(modifier.fillMaxWidth().height(1.dp).background(color))
}

/** A short vertical hairline between neighbouring items. */
@Composable
fun Divider(color: Color, height: Dp = 14.dp) {
    Box(Modifier.width(1.dp).height(height).background(color))
}
