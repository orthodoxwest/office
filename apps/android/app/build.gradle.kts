plugins {
    alias(libs.plugins.android.application)
    alias(libs.plugins.kotlin.compose)
    alias(libs.plugins.roborazzi)
}

// apps/android/app → the repository root, which holds the Cargo workspace and data/.
val repoRoot: File = rootDir.resolve("../..").canonicalFile
val ndkVersionPinned = "28.2.13676358"
val rustAbis = listOf("arm64-v8a", "armeabi-v7a", "x86_64")
val jniOut = layout.buildDirectory.dir("rustJniLibs")
val bindingsOut = layout.buildDirectory.dir("generated/uniffi")
val hostLibDir = repoRoot.resolve("target/release")

/** Commits on HEAD: a version code that only grows, the same on every machine. */
fun commitCount(): Int = providers.exec {
    workingDir = repoRoot
    commandLine("git", "rev-list", "--count", "HEAD")
}.standardOutput.asText.get().trim().toInt()

android {
    namespace = "org.orthodoxwest.office"
    compileSdk = 37
    ndkVersion = ndkVersionPinned

    defaultConfig {
        applicationId = "org.orthodoxwest.office"
        minSdk = 26
        targetSdk = 36
        versionCode = commitCount()
        versionName = "0.1.$versionCode"
    }

    signingConfigs {
        // A deliberately public key for sideloaded previews only, like Android's
        // debug key: every preview, from CI or a laptop, installs over the last.
        // Store releases will be signed by Play App Signing, never with this.
        create("preview") {
            storeFile = file("preview.keystore")
            storePassword = "preview"
            keyAlias = "preview"
            keyPassword = "preview"
        }
    }

    buildTypes {
        // Only builds that go to readers count usage (Usage.kt): never a debug build, its tests,
        // or its screenshots.
        debug {
            applicationIdSuffix = ".preview"
            signingConfig = signingConfigs.getByName("preview")
            buildConfigField("boolean", "COUNT_USAGE", "false")
        }
        // Shrunk by R8: the app's code is otherwise seven times the size of the whole
        // Rust engine, loaded on every launch and every reminder.
        release {
            isMinifyEnabled = true
            isShrinkResources = true
            proguardFiles(getDefaultProguardFile("proguard-android-optimize.txt"), "proguard-rules.pro")
            buildConfigField("boolean", "COUNT_USAGE", "true")
        }
        // Optimized and non-debuggable, so scrolling is as smooth as a release,
        // but installable beside the eventual store app.
        create("preview") {
            initWith(getByName("release"))
            applicationIdSuffix = ".preview"
            signingConfig = signingConfigs.getByName("preview")
            matchingFallbacks += "release"
        }
    }

    buildFeatures {
        compose = true
        buildConfig = true
    }

    sourceSets {
        getByName("main") {
            jniLibs.directories.add(jniOut.get().asFile.path)
            kotlin.directories.add(bindingsOut.get().asFile.path)
        }
    }

    testOptions {
        unitTests {
            isIncludeAndroidResources = true
            all {
                // Host tests call the same Rust core, built for the JVM's platform.
                it.systemProperty("jna.library.path", hostLibDir.path)
                it.systemProperty("roborazzi.test.record", "true")
            }
        }
    }
}

kotlin {
    jvmToolchain(21)
}

val cargoNdk = tasks.register<Exec>("cargoNdk") {
    description = "Builds the Rust core for each Android ABI."
    workingDir = repoRoot
    val ndkDir = androidComponents.sdkComponents.ndkDirectory
    doFirst { environment("ANDROID_NDK_HOME", ndkDir.get().asFile.path) }
    commandLine(
        listOf("cargo", "ndk") + rustAbis.flatMap { listOf("-t", it) } +
            listOf("--platform", "26", "-o", jniOut.get().asFile.path, "build", "--release", "--locked", "-p", "mobile-ffi", "--lib"),
    )
}

val cargoHost = tasks.register<Exec>("cargoHost") {
    description = "Builds the Rust core for this machine, for host tests and binding generation."
    workingDir = repoRoot
    commandLine("cargo", "build", "--release", "--locked", "-p", "mobile-ffi", "--lib")
}

val uniffiBindings = tasks.register<Exec>("uniffiBindings") {
    description = "Generates the Kotlin bindings from the host library."
    dependsOn(cargoHost)
    workingDir = repoRoot
    val lib = hostLibDir.resolve(System.mapLibraryName("office_mobile"))
    commandLine(
        "cargo", "run", "--release", "--locked", "-p", "mobile-ffi", "--features", "bindgen", "--bin", "uniffi-bindgen", "--",
        "generate", lib.path, "--language", "kotlin", "--out-dir", bindingsOut.get().asFile.path, "--no-format",
    )
}

tasks.named("preBuild") {
    dependsOn(cargoNdk, uniffiBindings)
}

dependencies {
    implementation(libs.androidx.core.ktx)
    implementation(libs.androidx.activity.compose)
    implementation(libs.androidx.lifecycle.viewmodel.compose)
    implementation(libs.androidx.lifecycle.runtime.compose)
    implementation(platform(libs.compose.bom))
    implementation(libs.compose.ui)
    implementation(libs.compose.ui.tooling.preview)
    implementation(libs.compose.material3)
    implementation("${libs.jna.get()}@aar")
    debugImplementation(libs.compose.ui.tooling)
    debugImplementation(libs.compose.ui.test.manifest)

    // The desktop JNA jar carries the host's native dispatch library.
    testImplementation(libs.jna)
    testImplementation(libs.junit)
    testImplementation(libs.robolectric)
    testImplementation(platform(libs.compose.bom))
    testImplementation(libs.compose.ui.test.junit4)
    testImplementation(libs.roborazzi)
    testImplementation(libs.roborazzi.compose)
    testImplementation(libs.roborazzi.junit.rule)
}
