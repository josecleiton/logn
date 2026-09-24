import SwiftUI

/// Termos de uso ou política de privacidade, numa folha.
///
/// O cabeçalho é nativo (voltar, título, versão e vigência) e o texto é a página do
/// servidor no modo do app. O rodapé com "Aceitar" só aparece quando quem abre pede
/// aceite — o caso da folha de aceite pendente, que ainda não existe; do cadastro, do
/// login e do perfil a tela é só leitura.
struct LegalDocumentView: View {
    @Environment(\.dismiss) private var dismiss

    @State private var kind: LegalKind
    /// Seções a marcar como novas. Vale para o documento aberto primeiro.
    var highlight: [String] = []
    var onAccept: (() -> Void)? = nil

    @State private var meta: LegalMeta?
    @State private var phase: LegalLoadPhase = .loading
    @State private var reloadToken = 0
    private let locale = AppLocale.current

    init(kind: LegalKind, highlight: [String] = [], onAccept: (() -> Void)? = nil) {
        _kind = State(initialValue: kind)
        self.highlight = highlight
        self.onAccept = onAccept
    }

    var body: some View {
        VStack(spacing: 0) {
            header

            ZStack {
                LegalWebView(
                    kind: kind, locale: locale, highlight: highlight, reloadToken: reloadToken,
                    meta: $meta, phase: $phase,
                    onSwitch: { kind = $0 }
                )
                .opacity(phase == .loaded ? 1 : 0)

                switch phase {
                case .loading: skeleton
                case .failed: failure
                case .loaded: EmptyView()
                }
            }

            if let onAccept {
                footer(onAccept)
            }
        }
        .background(LognDark.surfaceRaised.ignoresSafeArea())
        .preferredColorScheme(.dark)
    }

    // MARK: Cabeçalho

    private var header: some View {
        HStack(alignment: .center, spacing: 14) {
            Button { dismiss() } label: {
                Image(systemName: "chevron.left")
                    .font(.system(size: 16, weight: .semibold))
                    .foregroundColor(LognDark.textSecondary)
                    .frame(width: 44, height: 44)
            }
            .accessibilityLabel(Str.Logout.back)
            .padding(.leading, -12)

            VStack(alignment: .leading, spacing: 3) {
                Text(kind.title)
                    .font(.plexSansSemiBold(17, relativeTo: .headline))
                    .foregroundColor(LognDark.textPrimary)
                metaLine
            }

            Spacer(minLength: 0)
        }
        .padding(.horizontal, 20)
        .padding(.top, 10)
        .padding(.bottom, 12)
        .overlay(alignment: .bottom) {
            Rectangle().frame(height: 1).foregroundColor(LognDark.line)
        }
    }

    @ViewBuilder
    private var metaLine: some View {
        if let meta {
            // Uma linha cada: lado a lado, a data era cortada no meio em espanhol.
            VStack(alignment: .leading, spacing: 2) {
                // Rascunho não repete aqui: a própria página abre com a faixa.
                Text(Str.Legal.version_effective(meta.version, formattedDate(meta.effectiveAt)))
                if meta.offline {
                    Text(Str.Legal.offline_copy)
                }
            }
            .font(.plexMono(11, relativeTo: .caption))
            .foregroundColor(LognDark.textMuted)
            .fixedSize(horizontal: false, vertical: true)
        } else {
            // Esqueleto do tamanho da linha: o título não pula quando ela chega.
            RoundedRectangle(cornerRadius: Radius.xs)
                .fill(LognDark.line)
                .frame(width: 180, height: 10)
                .padding(.vertical, 3)
                .accessibilityHidden(true)
        }
    }

    /// A vigência por extenso, na língua do app.
    private func formattedDate(_ iso: String) -> String {
        let parser = DateFormatter()
        parser.locale = Locale(identifier: "en_US_POSIX")
        parser.timeZone = TimeZone(secondsFromGMT: -3 * 3600)
        parser.dateFormat = "yyyy-MM-dd"
        guard let date = parser.date(from: iso) else { return iso }
        var style = Date.FormatStyle(date: .long, time: .omitted).locale(Locale(identifier: locale))
        style.timeZone = TimeZone(secondsFromGMT: -3 * 3600) ?? .current
        return date.formatted(style)
    }

    // MARK: Estados

    /// Carregando: linhas no lugar do texto, sem spinner, como pede o design system.
    private var skeleton: some View {
        VStack(alignment: .leading, spacing: 12) {
            ForEach(0..<9, id: \.self) { i in
                RoundedRectangle(cornerRadius: Radius.xs)
                    .fill(LognDark.line)
                    .frame(height: i % 4 == 0 ? 14 : 10)
                    .frame(maxWidth: i % 4 == 3 ? 200 : .infinity, alignment: .leading)
                    .padding(.top, i % 4 == 0 && i > 0 ? 16 : 0)
            }
            Spacer()
        }
        .padding(20)
        .frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .topLeading)
        .background(LognDark.surfaceRaised)
        .accessibilityElement(children: .ignore)
        .accessibilityLabel(Str.Legal.loading_accessibility)
    }

    private var failure: some View {
        VStack(spacing: 16) {
            Text(Str.Legal.unavailable)
                .font(.plexSans(15, relativeTo: .body))
                .foregroundColor(LognDark.textSecondary)
                .multilineTextAlignment(.center)
            Button {
                reloadToken += 1
            } label: {
                Text(Str.Dashboard.try_again)
                    .font(.plexSansMedium(15))
                    .foregroundColor(LognDark.textPrimary)
                    .frame(minWidth: 160, minHeight: 48)
                    .overlay(RoundedRectangle(cornerRadius: Radius.sm).stroke(LognDark.lineStrong, lineWidth: 1))
            }
            .buttonStyle(PressSinkStyle())
        }
        .padding(.horizontal, 32)
        .frame(maxWidth: .infinity, maxHeight: .infinity)
        .background(LognDark.surfaceRaised)
    }

    // MARK: Rodapé

    private func footer(_ onAccept: @escaping () -> Void) -> some View {
        VStack(spacing: 0) {
            Rectangle().frame(height: 1).foregroundColor(LognDark.line)
            Button(action: onAccept) {
                Text(Str.Legal.accept)
                    .font(.plexSansSemiBold(15))
                    .foregroundColor(canAccept ? LognDark.onAccent : LognDark.textDim)
                    .frame(maxWidth: .infinity, minHeight: 52)
                    .background(canAccept ? LognDark.accent : LognDark.buttonDisabled)
                    .cornerRadius(Radius.sm)
            }
            .buttonStyle(PressSinkStyle())
            .disabled(!canAccept)
            .padding(.horizontal, 20)
            .padding(.top, 16)
            .padding(.bottom, 12)
        }
        .background(LognDark.surfaceRaised)
    }

    /// Só se aceita o que está na tela. Rascunho não se aceita, nem de cópia offline
    /// que pode estar velha.
    private var canAccept: Bool {
        phase == .loaded && meta.map { !$0.draft && !$0.offline } == true
    }
}

/// "Termos de uso · Política de privacidade", dois links que abrem a folha do documento.
struct LegalLinksRow: View {
    @Binding var presented: LegalKind?

    var body: some View {
        HStack(spacing: 4) {
            link(.terms)
            Text("·")
                .font(.plexSans(13))
                .foregroundColor(LognDark.textDim)
                .accessibilityHidden(true)
            link(.privacy)
        }
        .frame(maxWidth: .infinity)
    }

    private func link(_ kind: LegalKind) -> some View {
        Button { presented = kind } label: {
            Text(kind.title)
                .font(.plexSans(13, relativeTo: .footnote))
                .foregroundColor(LognDark.textSecondary)
                .underline()
                .frame(minHeight: 44)
                .padding(.horizontal, 6)
        }
        .buttonStyle(.plain)
        .accessibilityAddTraits(.isLink)
    }
}
