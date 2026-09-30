package sh.logn.app;

import android.app.Activity;
import android.graphics.Color;
import android.os.Bundle;
import android.view.Gravity;
import android.widget.TextView;

/**
 * Tela única do esqueleto: o nome do app sobre o fundo do design system. Não há texto
 * de interface aqui (regra 6); o nome da marca vem de {@code app_name}.
 *
 * <p>Em Java, sem dependência nenhuma, de propósito: o esqueleto existe só para subir o
 * pacote no Play Console. O cliente de verdade (Kotlin, Compose, o Core pela FFI) entra
 * por cima dele.
 */
public final class MainActivity extends Activity {
    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        TextView name = new TextView(this);
        name.setText(R.string.app_name);
        name.setTextSize(32);
        name.setTextColor(Color.parseColor("#EDEEEF"));
        name.setBackgroundColor(Color.parseColor("#0B0C0D"));
        name.setGravity(Gravity.CENTER);
        setContentView(name);
    }
}
