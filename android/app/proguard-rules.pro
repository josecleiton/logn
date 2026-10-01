# As regras da ponte com o Core vêm de :core-shell (consumer-rules.pro).

# review-ktx cita uma anotação do play-services que não vem com ele. É só anotação de
# compilação, sem efeito em execução.
-dontwarn com.google.android.gms.common.annotation.NoNullnessRewrite
