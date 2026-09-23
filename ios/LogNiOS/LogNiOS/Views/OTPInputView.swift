import SwiftUI
import LogN
import App

/// Os seis campos do código, e nada além disso.
///
/// A tela hospedeira já diz o que está pedindo e para qual e-mail — esta view repetia
/// o mesmo cabeçalho logo abaixo, e a mesma frase de status saía duas vezes, uma em
/// `warn` e outra em `info`. Aqui só o campo, a colagem e a confirmação.
///
/// A paleta também estava fora: foco e ação em `info` azul, e um envelope azul de 48pt
/// como ilustração. No LogN o acento é laranja e o azul é informação, não ação.
struct OTPInputView: View {
    @EnvironmentObject var core: CoreWrapper
    let email: String
    let purpose: String
    /// O código digitado, devolvido à tela hospedeira.
    ///
    /// Ele vivia só aqui dentro, e o cadastro mandava `otpEmail` no lugar do código:
    /// "Criar Conta" ia para o servidor com o e-mail no campo do OTP e voltava 401.
    @Binding var code: String

    @State private var digits: [String] = Array(repeating: "", count: 6)
    /// Último código já enviado, para não repetir a mesma submissão.
    @State private var lastSubmittedCode = ""
    @FocusState private var focusedIndex: Int?

    var otpCode: String { digits.joined() }

    var body: some View {
        VStack(spacing: Space.lg) {
            HStack(spacing: Space.sm) {
                ForEach(0..<6, id: \.self) { index in
                    TextField("", text: $digits[index])
                        .frame(width: 44, height: 56)
                        .multilineTextAlignment(.center)
                        .font(.plexMonoSemiBold(24))
                        .foregroundColor(LognDark.textPrimary)
                        .background(LognDark.surface)
                        .cornerRadius(Radius.sm)
                        .overlay(
                            RoundedRectangle(cornerRadius: Radius.sm)
                                .stroke(
                                    focusedIndex == index ? LognDark.accent : LognDark.line,
                                    lineWidth: focusedIndex == index ? 2 : 1
                                )
                        )
                        .keyboardType(.numberPad)
                        .focused($focusedIndex, equals: index)
                        .accessibilityLabel(Str.Otp.digit_accessibility(index + 1))
                        .onChange(of: digits[index]) { newVal in
                            if newVal.count > 1 {
                                digits[index] = String(newVal.last!)
                            }
                            if !newVal.isEmpty && index < 5 {
                                focusedIndex = index + 1
                            }
                            if otpCode.count == 6 && digits.allSatisfy({ !$0.isEmpty }) {
                                submitOTP()
                            }
                        }
                }
            }

            Button(action: pasteFromClipboard) {
                Label(Str.Otp.paste_code, systemImage: "doc.on.clipboard")
                    .lognLabel()
                    .foregroundColor(LognDark.textSecondary)
                    .frame(minHeight: Space.minTouch)
            }
            .buttonStyle(.plain)

            Button(action: { submitOTP(force: true) }) {
                Text(verifyLocked
                     ? Str.Status.wait_seconds(Int(core.viewModel.authCooldownSeconds))
                     : Str.Otp.verify)
                    .font(.plexSansSemiBold(15))
                    .monospacedDigit()
                    .foregroundColor(canVerify ? LognDark.onAccent : LognDark.textDim)
                    .frame(maxWidth: .infinity)
                    .frame(height: 52)
                    .background(canVerify ? LognDark.accent : LognDark.buttonDisabled)
                    .cornerRadius(Radius.sm)
            }
            .disabled(!canVerify || core.viewModel.isAuthenticating)

            Button(action: {
                core.dispatch(event: LogN.Event.requestOtp(email: email, purpose: purpose))
            }) {
                Text(resendLocked
                     ? Str.Status.wait_seconds(Int(core.viewModel.resendCooldownSeconds))
                     : Str.Otp.resend_code)
                    .font(.plexSans(13.5))
                    .monospacedDigit()
                    .foregroundColor(resendLocked ? LognDark.textDim : LognDark.textSecondary)
                    .underline(!resendLocked)
                    .frame(minHeight: Space.minTouch)
            }
            .buttonStyle(.plain)
            .disabled(resendLocked)
        }
        .onAppear { focusedIndex = 0 }
    }

    private var isComplete: Bool { otpCode.count == 6 }

    /// Travado por um 429. Verificar só trava no bloqueio geral: pedir código de novo
    /// cedo demais não invalida o que já chegou.
    private var verifyLocked: Bool { core.viewModel.authCooldownSeconds > 0 }
    private var resendLocked: Bool { core.viewModel.resendCooldownSeconds > 0 }
    private var canVerify: Bool { isComplete && !verifyLocked }

    private func pasteFromClipboard() {
        guard let clipboard = UIPasteboard.general.string else { return }
        let onlyDigits = clipboard.filter(\.isNumber)
        guard onlyDigits.count == 6 else { return }
        for (i, char) in onlyDigits.enumerated() {
            digits[i] = String(char)
        }
        submitOTP()
    }

    /// Envia o código uma única vez por valor.
    ///
    /// Preencher os seis campos — colando ou digitando — dispara `onChange` seis vezes,
    /// e a partir do último a condição "tudo preenchido" fica verdadeira em cada uma.
    /// Sem esta guarda o app mandava sete requisições por código, algumas com dígitos
    /// de uma tentativa anterior ainda no estado, e o servidor recusava as parciais.
    ///
    /// O toque no "Verificar" passa por cima da guarda (`force`): depois de um 429 o
    /// mesmo código precisa poder subir de novo, e antes ele ficava preso como "já
    /// enviado" mesmo sem o servidor ter chegado a conferi-lo.
    private func submitOTP(force: Bool = false) {
        let typed = otpCode
        guard typed.count == 6, force || typed != lastSubmittedCode else { return }
        guard !verifyLocked else { return }
        lastSubmittedCode = typed
        code = typed
        core.dispatch(event: LogN.Event.verifyOtp(email: email, code: typed, purpose: purpose))
    }
}
