import SwiftUI
import LogNCoreFFI
import LogN
import App

/// Escolher o apelido do placar (canvas "LogN — Placar geral de XP"): o campo, a
/// confirmação de que não dá para trocar depois, e o fim. Formato errado aparece no
/// "Continuar"; em uso e reservado, só depois de "Confirmar", e a folha volta ao campo.
struct NicknameSheet: View {
    @EnvironmentObject var core: CoreWrapper
    @Environment(\.colorScheme) private var colorScheme
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
                .fill(LognColors.lineStrong)
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
        .background(LognColors.surface.ignoresSafeArea())
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
                .foregroundColor(LognColors.textPrimary)
            bodyText(Str.Nickname.body(anon))
        }
        VStack(alignment: .leading, spacing: 8) {
            Text(Str.Nickname.label)
                .font(.plexMono(11))
                .tracking(0.1 * 11)
                .foregroundColor(LognColors.textMuted)
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
                .foregroundColor(hasError ? LognColors.warnInk : LognColors.textMuted)
                Spacer(minLength: 0)
                Text(Str.Nickname.counter(text.trimmingCharacters(in: .whitespaces).count, Self.maxLength))
                    .font(.plexMono(12))
                    .foregroundColor(LognColors.textMuted)
            }
        }
        sheetButton(Str.Nickname.continue_action, filled: true) {
            core.dispatch(event: .nicknameChecked(text))
        }
    }

    /// O exemplo dos outros campos é `textFaint`; este escrevia em `lineDim`, que no claro
    /// cai a 2:1. O escuro fica como estava (ADR 0031).
    private var placeholderColor: Color { colorScheme == .light ? LognColors.textFaint : LognColors.lineDim }

    private var field: some View {
        TextField("", text: $text, prompt: Text(Str.Nickname.placeholder).foregroundColor(placeholderColor))
            .font(.plexMono(16))
            .foregroundColor(LognColors.textPrimary)
            .textInputAutocapitalization(.never)
            .autocorrectionDisabled()
            .keyboardType(.asciiCapable)
            .submitLabel(.done)
            .onChange(of: text) { value in
                if value.count > Self.maxLength { text = String(value.prefix(Self.maxLength)) }
            }
            .padding(.horizontal, 14)
            .frame(height: 48)
            .background(LognColors.canvas)
            .cornerRadius(Radius.sm)
            .overlay(
                RoundedRectangle(cornerRadius: Radius.sm)
                    .stroke(hasError ? LognColors.warn : LognColors.lineStrong, lineWidth: 1)
            )
            .accessibilityLabel(Str.Nickname.field_accessibility)
    }

    // MARK: Confirmação

    @ViewBuilder
    private var confirmStep: some View {
        Text(Str.Nickname.confirm_eyebrow)
            .font(.plexMono(11))
            .tracking(0.1 * 11)
            .foregroundColor(LognColors.accentInk)
        VStack(alignment: .leading, spacing: 6) {
            Text(flow.draft)
                .font(.plexSansSemiBold(24))
                .foregroundColor(LognColors.textPrimary)
            Text(Str.Nickname.confirm_sub(anon))
                .font(.plexMono(12))
                .foregroundColor(LognColors.textSecondary)
                .fixedSize(horizontal: false, vertical: true)
        }
        .frame(maxWidth: .infinity, alignment: .leading)
        .padding(.horizontal, 16)
        .padding(.vertical, 18)
        .background(LognColors.accentTint)
        .cornerRadius(Radius.sm)
        .overlay(RoundedRectangle(cornerRadius: Radius.sm).stroke(LognColors.accent, lineWidth: 1))
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
                .foregroundColor(LognColors.textPrimary)
            bodyText(Str.Nickname.success_body)
        }
        sheetButton(Str.Nickname.close, filled: false, action: onClose)
    }

    // MARK: Peças

    private func bodyText(_ value: String) -> some View {
        Text(value)
            .font(.plexSans(14))
            .lineSpacing(bodyLineSpacing)
            .foregroundColor(LognColors.textSecondary)
            .fixedSize(horizontal: false, vertical: true)
    }

    private func sheetButton(_ title: String, filled: Bool, enabled: Bool = true, action: @escaping () -> Void) -> some View {
        Button(action: action) {
            Text(title)
                .font(filled ? .plexSansSemiBold(15) : .plexSansMedium(15))
                .foregroundColor(buttonInk(filled: filled, enabled: enabled))
                .frame(maxWidth: .infinity, minHeight: 48)
                .background(filled ? (enabled ? LognColors.accent : LognColors.buttonDisabled) : Color.clear)
                .cornerRadius(Radius.sm)
                .overlay(
                    RoundedRectangle(cornerRadius: Radius.sm)
                        .stroke(filled ? Color.clear : LognColors.lineStrong, lineWidth: 1)
                )
        }
        .buttonStyle(.plain)
        .disabled(!enabled)
    }

    private func buttonInk(filled: Bool, enabled: Bool) -> Color {
        if !enabled { return LognColors.textDim }
        return filled ? LognColors.onAccent : LognColors.textPrimary
    }
}
