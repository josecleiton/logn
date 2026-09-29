import SwiftUI
import LogNCoreFFI
import LogN
import App

/// Saída direta — quando não há nada na fila.
///
/// O DS diverge de propósito da spec técnica aqui: **sem alerta de confirmação**. Sair
/// estando sincronizado é reversível, entrar de novo devolve tudo, e alerta sem prêmio
/// treina o jogador a confirmar sem ler — o que destrói o valor do alerta que importa.
/// No lugar do alerta, esta tela, com desfazer.
struct LogoutNoticeView: View {
    @EnvironmentObject var core: CoreWrapper

    private var vm: LogN.ViewModel { core.viewModel }

    var body: some View {
        ZStack {
            LognDark.canvas.ignoresSafeArea()

            VStack(spacing: 0) {
                VStack(spacing: 16) {
                    Image(systemName: "rectangle.portrait.and.arrow.right")
                        .font(.system(size: 34, weight: .light))
                        .foregroundColor(LognDark.textMuted)

                    Text(farewell)
                        .font(.plexSansSemiBold(18, relativeTo: .headline))
                        .foregroundColor(LognDark.textPrimary)
                        .multilineTextAlignment(.center)

                    Text(Str.Logout.server_safe(Int(vm.globalXp)))
                        .font(.plexSans(14, relativeTo: .subheadline))
                        .lineSpacing(21 - 14)
                        .foregroundColor(LognDark.textSecondary)
                        .multilineTextAlignment(.center)
                        .fixedSize(horizontal: false, vertical: true)
                }
                .frame(maxHeight: .infinity)
                .padding(.horizontal, 34)

                VStack(spacing: 0) {
                    Button {
                        core.dispatch(event: .dismissLogoutNotice)
                    } label: {
                        Text(Str.Logout.sign_in_again)
                            .font(.plexSansSemiBold(15, relativeTo: .callout))
                            .foregroundColor(LognDark.onAccent)
                            .frame(maxWidth: .infinity, minHeight: 50)
                            .background(LognDark.accent)
                            .cornerRadius(Radius.sm)
                    }
                    .buttonStyle(PressSinkStyle())

                    Button {
                        core.dispatch(event: .undoLogout)
                    } label: {
                        HStack(spacing: 8) {
                            Image(systemName: "arrow.counterclockwise")
                                .font(.system(size: 11, weight: .medium))
                                .foregroundColor(LognDark.textMuted)
                            Text(Str.Logout.undo_logout)
                                .font(.plexMono(11.5))
                                .foregroundColor(LognDark.textSecondary)
                        }
                        .frame(maxWidth: .infinity, minHeight: 44)
                    }
                    .buttonStyle(.plain)
                    .padding(.top, 4)
                }
                .padding(.horizontal, 20)
                .padding(.bottom, 28)
            }
        }
    }

    private var farewell: String {
        vm.displayName.isEmpty ? Str.Logout.see_you : Str.Logout.see_you_name(vm.displayName)
    }
}

/// Saída com fila pendente.
///
/// Não pergunta "tem certeza": **nomeia a perda** em números, e põe a saída sem perda
/// como primário. O destrutivo fica em terceiro, como texto, repetindo verbo e número.
struct CriticalLogoutSheet: View {
    let pendingCount: Int
    let xpAtRisk: Int
    let onSyncAndLeave: () -> Void
    let onStay: () -> Void
    let onDiscard: () -> Void

    static let preferredHeight: CGFloat = 330

    var body: some View {
        ZStack(alignment: .top) {
            LognDark.surfaceRaised.ignoresSafeArea()

            VStack(alignment: .leading, spacing: 0) {
                Text(Str.Logout.risk_summary(Str.Profile.queued_events(pendingCount), xpAtRisk))
                    .font(.plexMono(10.5))
                    .tracking(0.14 * 10.5)
                    .foregroundColor(LognDark.warnInk)

                Text(Str.Logout.save_before)
                    .font(.plexSansSemiBold(18, relativeTo: .headline))
                    .lineSpacing(18 * 0.3)
                    .foregroundColor(LognDark.textPrimary)
                    .padding(.top, 9)

                Text(Str.Logout.save_desc)
                    .font(.plexSans(14, relativeTo: .subheadline))
                    .lineSpacing(21 - 14)
                    .foregroundColor(LognDark.textSecondary)
                    .fixedSize(horizontal: false, vertical: true)
                    .padding(.top, 9)

                Button(action: onSyncAndLeave) {
                    Text(Str.Logout.sync_and_leave)
                        .font(.plexSansSemiBold(15, relativeTo: .callout))
                        .foregroundColor(LognDark.onAccent)
                        .frame(maxWidth: .infinity, minHeight: 50)
                        .background(LognDark.accent)
                        .cornerRadius(Radius.sm)
                }
                .buttonStyle(PressSinkStyle())
                .padding(.top, 18)

                Button(action: onStay) {
                    Text(Str.Logout.stay_connected)
                        .font(.plexSansMedium(15))
                        .foregroundColor(LognDark.textPrimary)
                        .frame(maxWidth: .infinity, minHeight: 48)
                        .overlay(RoundedRectangle(cornerRadius: Radius.sm).stroke(LognDark.lineStrong, lineWidth: 1))
                }
                .buttonStyle(PressSinkStyle())
                .padding(.top, 8)

                // Texto, não botão: a perda deixa de ser o caminho padrão.
                Button(action: onDiscard) {
                    Text(Str.Logout.leave_and_drop(xpAtRisk))
                        .font(.plexSans(13.5, relativeTo: .footnote))
                        .foregroundColor(LognDark.wrongInk)
                        .frame(maxWidth: .infinity, minHeight: 42)
                }
                .buttonStyle(.plain)
                .padding(.top, 4)
            }
            .padding(.horizontal, 20)
            .padding(.top, 20)
            .padding(.bottom, 24)
        }
        .overlay(alignment: .top) {
            Rectangle().frame(height: 1).foregroundColor(LognDark.warn)
        }
    }
}

/// Gerenciar conta — tela empilhada, não sheet.
///
/// A exclusão vive aqui, um toque longe do logout: a App Store exige que seja
/// **encontrável**, não proeminente. Vizinha do "Sair", num sheet que o jogador abre
/// para ver XP, seria convite a acidente irreversível.
struct ManageAccountView: View {
    @EnvironmentObject var core: CoreWrapper
    @Environment(\.dismiss) private var dismiss
    @State private var showingDeleteConfirm = false

    var body: some View {
        ZStack {
            LognDark.canvas.ignoresSafeArea()

            VStack(alignment: .leading, spacing: 0) {
                header



                Spacer(minLength: 18)

                deletionBlock
                    .padding(.horizontal, 20)
                    .padding(.bottom, 20)
            }
        }
        .navigationBarHidden(true)
        .sheet(isPresented: $showingDeleteConfirm) {
            DeleteAccountSheet(
                onDelete: { password in
                    core.dispatch(event: .deleteAccount(passwordHash: password))
                },
                onDeleteWithProvider: { proof in
                    core.dispatch(event: .deleteAccountWithProvider(
                        provider: proof.provider, idToken: proof.idToken, nonce: proof.nonce,
                        authorizationCode: proof.authorizationCode))
                }
            )
            .environmentObject(core)
        }
    }

    private var header: some View {
        HStack(spacing: 14) {
            Button { dismiss() } label: {
                Image(systemName: "chevron.left")
                    .font(.system(size: 16, weight: .semibold))
                    .foregroundColor(LognDark.textSecondary)
            }
            .accessibilityLabel(Str.Logout.back)

            Text(Str.Account.manage)
                .font(.plexSansSemiBold(17))
                .foregroundColor(LognDark.textPrimary)

            Spacer(minLength: 0)
        }
        .padding(.horizontal, 20)
        .padding(.top, 18)
        .padding(.bottom, 16)
        .overlay(alignment: .bottom) {
            Rectangle().frame(height: 1).foregroundColor(LognDark.line)
        }
    }

    private func actionRow(_ title: String, sub: String?) -> some View {
        HStack(spacing: 12) {
            VStack(alignment: .leading, spacing: 0) {
                Text(title)
                    .font(.plexSansMedium(14.5))
                    .foregroundColor(LognDark.textPrimary)
                if let sub {
                    Text(sub)
                        .font(.plexMono(11))
                        .foregroundColor(LognDark.textMuted)
                        .lineLimit(1)
                        .padding(.top, 3)
                }
            }
            Spacer(minLength: 0)
            Image(systemName: "chevron.right")
                .font(.system(size: 12, weight: .medium))
                .foregroundColor(LognDark.textMuted)
        }
        .padding(.horizontal, 15)
        .padding(.vertical, 14)
        .background(LognDark.surface)
        .cornerRadius(Radius.sm)
        .overlay(RoundedRectangle(cornerRadius: Radius.sm).stroke(LognDark.line, lineWidth: 1))
    }

    /// Verde e vermelho só aparecem como veredito ou **ação destrutiva irreversível**,
    /// e nesse caso só dentro do bloco de confirmação — nunca no repouso da tela.
    private var deletionBlock: some View {
        VStack(alignment: .leading, spacing: 0) {
            Text(Str.Logout.grace_period)
                .font(.plexMono(10.5))
                .tracking(0.14 * 10.5)
                .foregroundColor(LognDark.wrongInk)

            Text(Str.Logout.delete_account)
                .font(.plexSansSemiBold(15, relativeTo: .callout))
                .foregroundColor(LognDark.textPrimary)
                .padding(.top, 8)

            Text(Str.Logout.delete_desc(Int(core.viewModel.globalXp)))
                .font(.plexSans(13, relativeTo: .footnote))
                .lineSpacing(19 - 13)
                .foregroundColor(LognDark.textSecondary)
                .fixedSize(horizontal: false, vertical: true)
                .padding(.top, 7)

            Button(action: { showingDeleteConfirm = true }) {
                Text(Str.Logout.delete_button)
                    .font(.plexSansSemiBold(14.5))
                    .foregroundColor(LognDark.wrongInk)
                    .frame(maxWidth: .infinity, minHeight: 46)
                    .overlay(RoundedRectangle(cornerRadius: Radius.sm).stroke(LognDark.wrong, lineWidth: 1))
            }
            .padding(.top, 14)

            Text(Str.Logout.delete_prompt)
                .font(.plexMono(10.5))
                .foregroundColor(LognDark.textMuted)
                .frame(maxWidth: .infinity)
                .padding(.top, 9)
        }
        .padding(16)
        .background(LognDark.tintErr)
        .cornerRadius(Radius.sm)
        .overlay(RoundedRectangle(cornerRadius: Radius.sm).stroke(LognDark.wrong, lineWidth: 1))
    }
}

/// A prova de dono que a exclusão manda quando não é pela senha.
struct ProviderProof {
    let provider: String
    let idToken: String
    let nonce: String
    /// Da Apple, para o servidor revogar o acesso. Vazio no Google.
    let authorizationCode: String
}

struct DeleteAccountSheet: View {
    let onDelete: (String) -> Void
    /// Conta criada por um provedor não tem senha: prova que é dona entrando nele de
    /// novo, na hora. O servidor só aceita login de poucos minutos atrás.
    let onDeleteWithProvider: (ProviderProof) -> Void
    @Environment(\.dismiss) private var dismiss
    @EnvironmentObject var core: CoreWrapper

    @State private var password = ""
    @State private var confirmation = ""
    @State private var providerBusy = false

    var body: some View {
        VStack(alignment: .leading, spacing: 20) {
            Text(Str.Logout.delete_account)
                .font(.plexSansSemiBold(18))
                .foregroundColor(LognDark.textPrimary)

            Text(Str.Logout.delete_desc(Int(core.viewModel.globalXp)))
                .font(.plexSans(14))
                .foregroundColor(LognDark.textSecondary)
                
            Text(Str.Logout.grace_period)
                .font(.plexSansSemiBold(14))
                .foregroundColor(LognDark.wrongInk)

            SecureField(Str.Login.password_prompt, text: $password)
                .font(.plexMono(14))
                .padding(.horizontal, 14)
                .frame(height: 52)
                .background(LognDark.surface)
                .cornerRadius(Radius.sm)
                .overlay(RoundedRectangle(cornerRadius: Radius.sm).stroke(LognDark.line, lineWidth: 1))
                .foregroundColor(LognDark.textPrimary)

            TextField(Str.Logout.delete_confirm_placeholder, text: $confirmation)
                .font(.plexMono(14))
                .padding(.horizontal, 14)
                .frame(height: 52)
                .background(LognDark.surface)
                .cornerRadius(Radius.sm)
                .overlay(RoundedRectangle(cornerRadius: Radius.sm).stroke(LognDark.line, lineWidth: 1))
                .foregroundColor(LognDark.textPrimary)

            Button(action: {
                if canDelete {
                    // Sobra de uma exclusão pelo Google que falhou não é revogada por esta.
                    GoogleAuth.shared.pendingRevocation = ""
                    onDelete(password)
                    dismiss()
                }
            }) {
                Text(Str.Logout.delete_button)
                    .font(.plexSansSemiBold(15))
                    .foregroundColor(canDelete ? LognDark.onAccent : LognDark.textDim)
                    .frame(maxWidth: .infinity, minHeight: 52)
                    .background(canDelete ? LognDark.wrong : LognDark.buttonDisabled)
                    .cornerRadius(Radius.sm)
            }
            .disabled(!canDelete)

            if AppleAuth.shared.isConfigured {
                providerButton(Str.Logout.delete_with_apple, action: deleteWithApple)
            }
            if GoogleAuth.shared.isConfigured {
                providerButton(Str.Logout.delete_with_google, action: deleteWithGoogle)
            }
        }
        .padding(20)
        .background(LognDark.surfaceRaised)
        .preferredColorScheme(.dark)
    }

    private var confirmed: Bool { confirmation == "EXCLUIR" }

    private var canDelete: Bool {
        return !password.isEmpty && confirmed
    }

    @ViewBuilder
    private func providerButton(_ title: String, action: @escaping () -> Void) -> some View {
        Button(action: action) {
            Text(title)
                .font(.plexSans(14))
                .foregroundColor(confirmed ? LognDark.textSecondary : LognDark.textDim)
                .frame(maxWidth: .infinity, minHeight: 44)
        }
        .buttonStyle(.plain)
        .disabled(!confirmed || providerBusy)
    }

    private func deleteWithGoogle() {
        providerBusy = true
        Task {
            defer { providerBusy = false }
            do {
                let credential = try await GoogleAuth.shared.signIn()
                // Revogado só se a exclusão der certo: a tela "conta desativada" dispara.
                GoogleAuth.shared.pendingRevocation = credential.accessToken
                onDeleteWithProvider(ProviderProof(
                    provider: "google", idToken: credential.idToken, nonce: credential.nonce, authorizationCode: ""))
                dismiss()
            } catch GoogleAuth.Failure.cancelled {
                // Desistiu na janela do Google: a conta fica.
            } catch {
                core.dispatch(event: .socialLoginFailed)
            }
        }
    }

    private func deleteWithApple() {
        providerBusy = true
        Task {
            defer { providerBusy = false }
            do {
                let credential = try await AppleAuth.shared.signIn()
                GoogleAuth.shared.pendingRevocation = ""
                // O servidor troca o código e revoga na Apple; o aparelho não faz nada.
                onDeleteWithProvider(ProviderProof(
                    provider: "apple", idToken: credential.idToken, nonce: credential.nonce,
                    authorizationCode: credential.authorizationCode))
                dismiss()
            } catch AppleAuth.Failure.cancelled {
                // Desistiu na folha da Apple: a conta fica.
            } catch {
                core.dispatch(event: .socialLoginFailed)
            }
        }
    }
}
