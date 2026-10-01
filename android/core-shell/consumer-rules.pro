# O glue JNI resolve classe e método pelo nome em tempo de execução: R8 não pode
# renomear nem tirar nada do pacote da ponte.
-keep class sh.logn.ffi.** { *; }
-keepclasseswithmembernames,includedescriptorclasses class * {
    native <methods>;
}
# Os tipos do Core. Dá para afrouxar depois de o smoke de release passar sem eles.
-keep class sh.logn.core.** { *; }
