import SwiftUI
import LogNCoreFFI
import LogN
import App

/// Escolher o apelido do placar (canvas "LogN — Placar geral de XP"): o campo, a
/// confirmação de que não dá para trocar depois, e o fim. Formato errado aparece no
/// "Continuar"; em uso e reservado, só depois de "Confirmar", e a folha volta ao campo.
struct NicknameSheet: View {
    @EnvironmentObject var core: CoreWrapper
    let onClose: () -> Void

    @State private var text = ""
    /// A folha abraça o conteúdo, como no canvas: cada passo tem a sua altura.
    @State private var contentHeight: CGFloat = 400

    private struct ContentHeightKey: PreferenceKey {
        static var defaultValue: CGFloat = 0
        static func reduce(value: inout CGFloat, nextValue: () -> CGFloat) {
            value = max(value, nextValue())
        }
    }

    private static let maxLength = 20

    private var flow: NicknameFlowView { core.viewModel.nicknameFlow }
    private var anon: String { Str.Leaderboard.anon_name(String(core.viewModel.profileAnonNumber)) }

    var body: some View {
        VStack(alignment: .leading, spacing: 18) {
            Capsule()
                .fill(LognDark.lineStrong)
                .frame(width: 36, height: 4)
                .frame(maxWidth: .infinity)
            step
        }
        .padding(.horizontal, Space.screenMargin)
        .padding(.top, 12)
        .padding(.bottom, 28)
        .background(
            GeometryReader { proxy in
                Color.clear.preference(key: ContentHeightKey.self, value: proxy.size.height)
            }
        )
        .frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .top)
        .background(LognDark.surface.ignoresSafeArea())
        .onPreferenceChange(ContentHeightKey.self) { height in
            if height > 0 { contentHeight = height }
        }
        .presentationDetents([.height(contentHeight)])
        .interactiveDismissDisabled(flow.submitting)
        .onAppear { core.dispatch(event: .nicknameStarted) }
        .onDisappear { core.dispatch(event: .nicknameFlowClosed) }
    }

    @ViewBuilder
    private var step: some View {
        switch flow.step {
        case .input:   inputStep
        case .confirm: confirmStep
        case .done:    doneStep
        }
    }

    // MARK: Campo

    private var hasError: Bool { flow.error != .silent }

    private var helperText: String {
        hasError ? (flow.error.copy ?? Str.Nickname.helper) : Str.Nickname.helper
    }

    @ViewBuilder
    private var inputStep: some View {
        VStack(alignment: .leading, spacing: 8) {
            Text(Str.Nickname.title)
                .font(.plexSansSemiBold(20))
                .foregroundColor(LognDark.textPrimary)
            bodyText(Str.Nickname.body(anon))
        }
        VStack(alignment: .leading, spacing: 8) {
            Text(Str.Nickname.label)
                .font(.plexMono(11))
                .tracking(0.1 * 11)
                .foregroundColor(LognDark.textMuted)
            field
            HStack(alignment: .top, spacing: 12) {
                HStack(alignment: .top, spacing: 6) {
                    if hasError {
                        Image(systemName: "exclamationmark.circle")
                            .font(.system(size: 12, weight: .semibold))
                            .padding(.top, 2)
                    }
                    Text(helperText)
                        .font(.plexSans(13))
                        .fixedSize(horizontal: false, vertical: true)
                }
                .foregroundColor(hasError ? LognDark.warnInk : LognDark.textMuted)
                Spacer(minLength: 0)
                Text(Str.Nickname.counter(text.trimmingCharacters(in: .whitespaces).count, Self.maxLength))
                    .font(.plexMono(12))
                    .foregroundColor(LognDark.textMuted)
            }
        }
        sheetButton(Str.Nickname.continue_action, filled: true) {
            core.dispatch(event: .nicknameChecked(text))
        }
    }

    private var field: some View {
        TextField("", text: $text, prompt: Text(Str.Nickname.placeholder).foregroundColor(LognDark.lineDim))
            .font(.plexMono(16))
            .foregroundColor(LognDark.textPrimary)
            .textInputAutocapitalization(.never)
            .autocorrectionDisabled()
            .keyboardType(.asciiCapable)
            .submitLabel(.done)
            .onChange(of: text) { value in
                if value.count > Self.maxLength { text = String(value.prefix(Self.maxLength)) }
            }
            .padding(.horizontal, 14)
            .frame(height: 48)
            .background(LognDark.canvas)
            .cornerRadius(Radius.sm)
            .overlay(
                RoundedRectangle(cornerRadius: Radius.sm)
                    .stroke(hasError ? LognDark.warn : LognDark.lineStrong, lineWidth: 1)
            )
            .accessibilityLabel(Str.Nickname.field_accessibility)
    }

    // MARK: Confirmação

    @ViewBuilder
    private var confirmStep: some View {
        Text(Str.Nickname.confirm_eyebrow)
            .font(.plexMono(11))
            .tracking(0.1 * 11)
            .foregroundColor(LognDark.accentInk)
        VStack(alignment: .leading, spacing: 6) {
            Text(flow.draft)
                .font(.plexSansSemiBold(24))
                .foregroundColor(LognDark.textPrimary)
            Text(Str.Nickname.confirm_sub(anon))
                .font(.plexMono(12))
                .foregroundColor(LognDark.textSecondary)
                .fixedSize(horizontal: false, vertical: true)
        }
        .frame(maxWidth: .infinity, alignment: .leading)
        .padding(.horizontal, 16)
        .padding(.vertical, 18)
        .background(LognDark.accentTint)
        .cornerRadius(Radius.sm)
        .overlay(RoundedRectangle(cornerRadius: Radius.sm).stroke(LognDark.accent, lineWidth: 1))
        bodyText(Str.Nickname.confirm_body)
        VStack(spacing: 10) {
            sheetButton(Str.Nickname.confirm(flow.draft), filled: true, enabled: !flow.submitting) {
                core.dispatch(event: .nicknameSubmitted(flow.draft))
            }
            sheetButton(Str.Nickname.back, filled: false, enabled: !flow.submitting) {
                core.dispatch(event: .nicknameBack)
            }
        }
    }

    // MARK: Fim

    @ViewBuilder
    private var doneStep: some View {
        VStack(alignment: .leading, spacing: 8) {
            Text(Str.Nickname.success(flow.draft))
                .font(.plexSansSemiBold(20))
                .foregroundColor(LognDark.textPrimary)
            bodyText(Str.Nickname.success_body)
        }
        sheetButton(Str.Nickname.close, filled: false, action: onClose)
    }

    // MARK: Peças

    private func bodyText(_ value: String) -> some View {
        Text(value)
            .font(.plexSans(14))
            .lineSpacing(bodyLineSpacing)
            .foregroundColor(LognDark.textSecondary)
            .fixedSize(horizontal: false, vertical: true)
    }

    private func sheetButton(_ title: String, filled: Bool, enabled: Bool = true, action: @escaping () -> Void) -> some View {
        Button(action: action) {
            Text(title)
                .font(filled ? .plexSansSemiBold(15) : .plexSansMedium(15))
                .foregroundColor(buttonInk(filled: filled, enabled: enabled))
                .frame(maxWidth: .infinity, minHeight: 48)
                .background(filled ? (enabled ? LognDark.accent : LognDark.buttonDisabled) : Color.clear)
                .cornerRadius(Radius.sm)
                .overlay(
                    RoundedRectangle(cornerRadius: Radius.sm)
                        .stroke(filled ? Color.clear : LognDark.lineStrong, lineWidth: 1)
                )
        }
        .buttonStyle(.plain)
        .disabled(!enabled)
    }

    private func buttonInk(filled: Bool, enabled: Bool) -> Color {
        if !enabled { return LognDark.textDim }
        return filled ? LognDark.onAccent : LognDark.textPrimary
    }
}
