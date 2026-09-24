import SwiftUI
import LogNCoreFFI
import LogN

/// De onde o problema veio, quando não foi escrito para o LogN.
///
/// O Core manda a chave da origem — a coluna `origin` do desafio — e a cópia vive em
/// `i18n/locales/`, como toda cópia do app. A primeira leitura de cada origem segura o
/// relógio da questão, e o cartão avisa que isso acontece uma vez só: sem o aviso, quem
/// abrisse de novo perderia tempo sem entender por quê.
struct OriginSheetView: View {
    let origin: String
    let clockPaused: Bool
    let onClose: () -> Void

    var body: some View {
        VStack(alignment: .leading, spacing: 0) {
            // O texto rola e o botão fica ancorado: a homenagem é longa e, presa a uma
            // altura fixa, ela cortava o nome no topo em vez de deixar ler até o fim.
            ScrollView {
                content
            }

            Button(action: onClose) {
                Text(Str.Origin.close)
                    .font(.plexSansSemiBold(16, relativeTo: .body))
                    .foregroundColor(LognDark.onAccent)
                    .frame(maxWidth: .infinity)
                    .padding(.vertical, 16)
                    .background(LognDark.accent)
                    .cornerRadius(Radius.sm)
            }
            .padding(.horizontal, 24)
            .padding(.top, 16)
            .padding(.bottom, 24)
        }
        .frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .topLeading)
        .background(LognDark.surface)
    }

    private var content: some View {
        VStack(alignment: .leading, spacing: 0) {
            Text(Str.Origin.eyebrow.uppercased())
                .font(.plexMono(11))
                .tracking(0.12 * 11)
                .foregroundColor(LognDark.textMuted)

            Text(name)
                .font(.plexSansSemiBold(28, relativeTo: .title))
                .foregroundColor(LognDark.textPrimary)
                .padding(.top, 10)

            Text(role)
                .font(.plexMono(12))
                .tracking(0.08 * 12)
                .foregroundColor(LognDark.accent)
                .padding(.top, 4)

            // A régua fica entre o nome e o texto: separa a identificação da história
            // sem fechar um bloco, que é o que uma moldura faria.
            Rectangle()
                .frame(height: 1)
                .foregroundColor(LognDark.line)
                .padding(.top, 20)

            Text(body_)
                .font(.plexSans(16, relativeTo: .body))
                .lineSpacing(16 * 0.45)
                .foregroundColor(LognDark.textSecondary)
                .fixedSize(horizontal: false, vertical: true)
                .padding(.top, 20)

            if clockPaused {
                pauseNotice.padding(.top, 24)
            }
        }
        .padding(.horizontal, 24)
        // 44 no topo, não 32: o puxador do sheet ocupa a primeira faixa e comia a tarja.
        .padding(.top, 44)
        .frame(maxWidth: .infinity, alignment: .topLeading)
    }

    /// O aviso da pausa é informação, não alarme: usa a cor de informação do DS e uma
    /// superfície elevada, não a de erro.
    private var pauseNotice: some View {
        HStack(alignment: .top, spacing: 10) {
            Image(systemName: "pause.circle")
                .font(.system(size: 15, weight: .medium))
                .foregroundColor(LognDark.info)

            Text(Str.Origin.pause_notice)
                .font(.plexMono(12))
                .lineSpacing(12 * 0.4)
                .foregroundColor(LognDark.textSecondary)
                .fixedSize(horizontal: false, vertical: true)
        }
        .padding(14)
        .frame(maxWidth: .infinity, alignment: .leading)
        .background(LognDark.surfaceRaised)
        .overlay(
            RoundedRectangle(cornerRadius: Radius.sm)
                .stroke(LognDark.info.opacity(0.3), lineWidth: 1)
        )
        .cornerRadius(Radius.sm)
    }

    // MARK: - Cópia por origem

    // Só há uma origem hoje. Quando houver a segunda, isto vira um switch com uma
    // entrada por chave — e a cópia continua vindo do i18n, não daqui.
    private var name: String { origin == "FARIAS" ? Str.Origin.origin_name : origin }
    private var role: String { origin == "FARIAS" ? Str.Origin.origin_role : "" }
    private var body_: String { origin == "FARIAS" ? Str.Origin.origin_body : "" }
}
