# JNA binds the generated UniFFI bindings to the Rust library by reflection: class, method and
# field names must survive exactly as written. JNA's AAR ships no rules of its own.
-keep class com.sun.jna.** { *; }
-keep class * implements com.sun.jna.** { *; }
-keep class org.orthodoxwest.office.core.** { *; }
# neuma's bindings, for chant scores, ride in the same library.
-keep class org.orthodoxwest.neuma.** { *; }
# Desktop-only corners of JNA, never loaded on Android.
-dontwarn java.awt.**
