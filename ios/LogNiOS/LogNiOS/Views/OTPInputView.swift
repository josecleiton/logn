import SwiftUI
import LogN
import App

struct OTPInputView: View {
    @EnvironmentObject var core: CoreWrapper
    let email: String
    let purpose: String
    
    @State private var digits: [String] = Array(repeating: "", count: 6)
    @FocusState private var focusedIndex: Int?
    
    var otpCode: String { digits.joined() }
    
    var body: some View {
        ZStack {
            LognDark.canvas.ignoresSafeArea()
            
            VStack(spacing: Space.xl) {
                Image(systemName: "envelope.badge.shield.half.filled")
                    .font(.system(size: 48))
                    .foregroundColor(LognDark.infoInk)
                
                Text("Código de verificação")
                    .font(LognFont.headlineMedium)
                    .foregroundColor(LognDark.textPrimary)
                
                Text("Digite o código de 6 dígitos enviado para\n\(email)")
                    .font(LognFont.bodyLarge)
                    .foregroundColor(LognDark.textSecondary)
                    .multilineTextAlignment(.center)
                
                // OTP Input Boxes
                HStack(spacing: Space.sm) {
                    ForEach(0..<6, id: \.self) { index in
                        TextField("", text: $digits[index])
                            .frame(width: 48, height: 56)
                            .multilineTextAlignment(.center)
                            .font(.plexMonoSemiBold(24))
                            .foregroundColor(LognDark.textPrimary)
                            .background(LognDark.surface)
                            .cornerRadius(Radius.sm)
                            .overlay(
                                RoundedRectangle(cornerRadius: Radius.sm)
                                    .stroke(focusedIndex == index ? LognDark.info : LognDark.lineDim, lineWidth: 2)
                            )
                            .keyboardType(.numberPad)
                            .focused($focusedIndex, equals: index)
                            .onChange(of: digits[index]) { newVal in
                                // Limit to 1 digit
                                if newVal.count > 1 {
                                    digits[index] = String(newVal.last!)
                                }
                                // Auto-advance
                                if !newVal.isEmpty && index < 5 {
                                    focusedIndex = index + 1
                                }
                                // Auto-submit when all 6 are filled
                                if otpCode.count == 6 && digits.allSatisfy({ !$0.isEmpty }) {
                                    submitOTP()
                                }
                            }
                    }
                }
                
                // Paste button
                Button(action: {
                    if let clipboardContent = UIPasteboard.general.string,
                       clipboardContent.count == 6,
                       clipboardContent.allSatisfy({ $0.isNumber }) {
                        for (i, char) in clipboardContent.enumerated() {
                            digits[i] = String(char)
                        }
                        submitOTP()
                    }
                }) {
                    Label("Colar da área de transferência", systemImage: "doc.on.clipboard")
                        .lognLabel()
                        .foregroundColor(LognDark.infoInk)
                }
                .padding(.top, Space.sm)
                
                // Verify Button
                Button(action: { submitOTP() }) {
                    Text("Verificar")
                        .font(LognFont.titleMedium)
                        .frame(maxWidth: .infinity)
                        .padding(.vertical, Space.md)
                        // Verde é veredito do juiz. Ação neutra de formulário usa o acento.
                        .background(otpCode.count == 6 ? LognDark.accent : LognDark.buttonDisabled)
                        .foregroundColor(otpCode.count == 6 ? LognDark.onAccent : LognDark.textDim)
                        .cornerRadius(Radius.sm)
                }
                .disabled(otpCode.count < 6 || core.viewModel.isAuthenticating)
                
                // Resend
                Button(action: {
                    core.dispatch(event: LogN.Event.requestOtp(email: email, purpose: purpose))
                }) {
                    Text("Reenviar código")
                        .font(LognFont.bodyLarge)
                        .foregroundColor(LognDark.textSecondary)
                        .underline()
                }
                
                if !core.viewModel.displayStatus.isEmpty {
                    Text(core.viewModel.displayStatus)
                        .lognLabel()
                        .foregroundColor(core.viewModel.otpVerified ? LognDark.correctInk : LognDark.warnInk)
                        .padding(.top, Space.sm)
                }
            }
            .padding(.horizontal, Space.screenMargin)
        }
        .onAppear {
            focusedIndex = 0
        }
    }
    
    private func submitOTP() {
        core.dispatch(event: LogN.Event.verifyOtp(email: email, code: otpCode, purpose: purpose))
    }
}
