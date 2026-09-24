import SwiftUI
import LogNCoreFFI
import LogN

/// A abertura do app: o juiz rodando os testes.
///
/// Enquanto o balão sobe, o Core confere a sessão e manda a fila, e cada verificação
/// imprime o seu veredito numa linha. Não há duração mínima: a splash some quando a
/// última linha fecha. Quem decide tudo é o Core; aqui só se desenha o `BootViewModel`.
///
/// Sem rede e com a sessão dentro do prazo, ela para e pergunta: tentar de novo, ou
/// entrar com o progresso do aparelho.
struct SplashView: View {
    @EnvironmentObject var core: CoreWrapper
    @Environment(\.accessibilityReduceMotion) private var reduceMotion

    @State private var risen = false

    private var boot: LogN.BootViewModel { core.viewModel.boot }

    var body: some View {
        ZStack {
            LognDark.canvas.ignoresSafeArea()

            VStack(spacing: 0) {
                Spacer()

                VStack(spacing: Space.md) {
                    BrandSymbol(color: LognDark.accent, height: 72)
                        .offset(y: risen ? 0 : 28)
                    Text("LogN")
                        .font(.plexSansSemiBold(28))
                        .tracking(-0.035 * 28)
                        .foregroundColor(LognDark.textPrimary)
                }
                .accessibilityElement(children: .ignore)
                .accessibilityLabel(Str.Boot.opening_accessibility)

                Spacer()

                log

                if boot.awaitingOfflineChoice {
                    offlineChoice
                        .padding(.top, Space.lg)
                        .transition(.opacity)
                } else {
                    progressBar
                        .padding(.top, Space.md)
                }
            }
            .padding(.horizontal, 20)
            .padding(.bottom, Space.xl)
        }
        .onAppear {
            // Reduce Motion: o balão já nasce no lugar.
            if reduceMotion {
                risen = true
            } else {
                withAnimation(.easeOut(duration: 0.9)) { risen = true }
            }
        }
        .animation(reduceMotion ? nil : .easeOut(duration: 0.2), value: boot.awaitingOfflineChoice)
    }

    private var log: some View {
        VStack(alignment: .leading, spacing: 6) {
            ForEach(Array(boot.lines.enumerated()), id: \.offset) { _, line in
                HStack(spacing: Space.sm) {
                    Image(systemName: line.symbol)
                        .font(.system(size: 10, weight: .bold))
                        .foregroundColor(line.tone)
                        .frame(width: 12, alignment: .leading)
                    Text(line.checkLabel)
                        .foregroundColor(LognDark.textPrimary)
                    Spacer(minLength: Space.sm)
                    Text(line.detailLabel)
                        .foregroundColor(LognDark.textMuted)
                        .lineLimit(1)
                        .minimumScaleFactor(0.8)
                }
                .font(.plexMono(12, relativeTo: .caption))
                .accessibilityElement(children: .ignore)
                .accessibilityLabel(Str.Boot.line_accessibility(line.checkLabel, line.detailLabel))
            }
        }
        .frame(maxWidth: .infinity, alignment: .leading)
    }

    private var progressBar: some View {
        GeometryReader { geo in
            ZStack(alignment: .leading) {
                Rectangle().fill(LognDark.line)
                Rectangle()
                    .fill(LognDark.accent)
                    .frame(width: geo.size.width * CGFloat(boot.progress) / 100)
                    .animation(reduceMotion ? nil : .easeOut(duration: 0.3), value: boot.progress)
            }
        }
        .frame(height: 2)
        .accessibilityHidden(true)
    }

    private var offlineChoice: some View {
        VStack(spacing: Space.md) {
            Text(Str.Boot.offline_notice)
                .font(.plexSans(13, relativeTo: .footnote))
                .foregroundColor(LognDark.textPrimary)
                .fixedSize(horizontal: false, vertical: true)
                .frame(maxWidth: .infinity, alignment: .leading)
                .padding(Space.md)
                .background(LognDark.tintWarn)
                .overlay(RoundedRectangle(cornerRadius: Radius.sm).stroke(LognDark.warn, lineWidth: 1))
                .cornerRadius(Radius.sm)

            HStack(spacing: Space.sm) {
                LognButton(title: Str.Boot.retry, variant: .secondary) {
                    core.dispatch(event: .retryBoot)
                }
                LognButton(title: Str.Boot.continue_offline, variant: .primary) {
                    core.dispatch(event: .continueOffline)
                }
            }
        }
    }
}
