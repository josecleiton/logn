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
            LognColors.canvas.ignoresSafeArea()

            VStack(spacing: 0) {
                VStack(spacing: 16) {
                    Image(systemName: "rectangle.portrait.and.arrow.right")
                        .font(.system(size: 34, weight: .light))
                        .foregroundColor(LognColors.textMuted)

                    Text(farewell)
                        .font(.plexSansSemiBold(18, relativeTo: .headline))
                        .foregroundColor(LognColors.textPrimary)
                        .multilineTextAlignment(.center)

                    Text(Str.Logout.server_safe(Int(vm.globalXp)))
                        .font(.plexSans(14, relativeTo: .subheadline))
                        .lineSpacing(21 - 14)
                        .foregroundColor(LognColors.textSecondary)
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
                            .foregroundColor(LognColors.onAccent)
                            .frame(maxWidth: .infinity, minHeight: 50)
                            .background(LognColors.accent)
                            .cornerRadius(Radius.sm)
                    }
                    .buttonStyle(PressSinkStyle())

                    Button {
                        core.dispatch(event: .undoLogout)
                    } label: {
                        HStack(spacing: 8) {
                            Image(systemName: "arrow.counterclockwise")
                                .font(.system(size: 11, weight: .medium))
                                .foregroundColor(LognColors.textMuted)
                            Text(Str.Logout.undo_logout)
                                .font(.plexMono(11.5))
                                .foregroundColor(LognColors.textSecondary)
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
    /// A fila está subindo: a saída espera a resposta, e o botão diz isso.
    let isSyncing: Bool
    let onSyncAndLeave: () -> Void
    let onStay: () -> Void
    let onDiscard: () -> Void

    static let preferredHeight: CGFloat = 330

    var body: some View {
        ZStack(alignment: .top) {
            LognColors.surfaceRaised.ignoresSafeArea()

            VStack(alignment: .leading, spacing: 0) {
                Text(Str.Logout.risk_summary(Str.Profile.queued_events(pendingCount), xpAtRisk))
                    .font(.plexMono(10.5))
                    .tracking(0.14 * 10.5)
                    .foregroundColor(LognColors.warnInk)

                Text(Str.Logout.save_before)
                    .font(.plexSansSemiBold(18, relativeTo: .headline))
                    .lineSpacing(18 * 0.3)
                    .foregroundColor(LognColors.textPrimary)
                    .padding(.top, 9)

                Text(Str.Logout.save_desc)
                    .font(.plexSans(14, relativeTo: .subheadline))
                    .lineSpacing(21 - 14)
                    .foregroundColor(LognColors.textSecondary)
                    .fixedSize(horizontal: false, vertical: true)
                    .padding(.top, 9)

                Button(action: onSyncAndLeave) {
                    Group {
                        if isSyncing {
                            ProgressView()
                                .progressViewStyle(CircularProgressViewStyle(tint: LognColors.onAccent))
                        } else {
                            Text(Str.Logout.sync_and_leave)
                                .font(.plexSansSemiBold(15, relativeTo: .callout))
                        }
                    }
                    .foregroundColor(LognColors.onAccent)
                    .frame(maxWidth: .infinity, minHeight: 50)
                    .background(LognColors.accent)
                    .cornerRadius(Radius.sm)
                }
                .buttonStyle(PressSinkStyle())
                .disabled(isSyncing)
                .accessibilityLabel(Str.Logout.sync_and_leave)
                .padding(.top, 18)

                Button(action: onStay) {
                    Text(Str.Logout.stay_connected)
                        .font(.plexSansMedium(15))
                        .foregroundColor(LognColors.textPrimary)
                        .frame(maxWidth: .infinity, minHeight: 48)
                        .overlay(RoundedRectangle(cornerRadius: Radius.sm).stroke(LognColors.lineStrong, lineWidth: 1))
                }
                .buttonStyle(PressSinkStyle())
                .padding(.top, 8)

                // Texto, não botão: a perda deixa de ser o caminho padrão.
                Button(action: onDiscard) {
                    Text(Str.Logout.leave_and_drop(xpAtRisk))
                        .font(.plexSans(13.5, relativeTo: .footnote))
                        .foregroundColor(LognColors.wrongInk)
                        .frame(maxWidth: .infinity, minHeight: 42)
                }
                .buttonStyle(.plain)
                // Descartar com o envio no ar apagaria a fila que está subindo.
                .disabled(isSyncing)
                .padding(.top, 4)
            }
            .padding(.horizontal, 20)
            .padding(.top, 20)
            .padding(.bottom, 24)
        }
        .overlay(alignment: .top) {
            Rectangle().frame(height: 1).foregroundColor(LognColors.warn)
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
            LognColors.canvas.ignoresSafeArea()

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
                    core.dispatch(event: .deleteAccount(password: password))
                },
                onDeleteWithProvider: { proof in
                    core.dispatch(event: .deleteAccountWithProvider(
                        provider: proof.provider, idToken: proof.idToken, nonce: proof.nonce,
                        authorizationCode: proof.authorizationCode))
                },
                onDeleteWithGitHub: { credential in
                    core.dispatch(event: .deleteAccountWithGitHub(
                        code: credential.code, codeVerifier: credential.verifier, nonce: credential.nonce))
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
                    .foregroundColor(LognColors.textSecondary)
            }
            .accessibilityLabel(Str.Logout.back)

            Text(Str.Account.manage)
                .font(.plexSansSemiBold(17))
                .foregroundColor(LognColors.textPrimary)

            Spacer(minLength: 0)
        }
        .padding(.horizontal, 20)
        .padding(.top, 18)
        .padding(.bottom, 16)
        .overlay(alignment: .bottom) {
            Rectangle().frame(height: 1).foregroundColor(LognColors.line)
        }
    }

    private func actionRow(_ title: String, sub: String?) -> some View {
        HStack(spacing: 12) {
            VStack(alignment: .leading, spacing: 0) {
                Text(title)
                    .font(.plexSansMedium(14.5))
                    .foregroundColor(LognColors.textPrimary)
                if let sub {
                    Text(sub)
                        .font(.plexMono(11))
                        .foregroundColor(LognColors.textMuted)
                        .lineLimit(1)
                        .padding(.top, 3)
                }
            }
            Spacer(minLength: 0)
            Image(systemName: "chevron.right")
                .font(.system(size: 12, weight: .medium))
                .foregroundColor(LognColors.textMuted)
        }
        .padding(.horizontal, 15)
        .padding(.vertical, 14)
        .background(LognColors.surface)
        .cornerRadius(Radius.sm)
        .overlay(RoundedRectangle(cornerRadius: Radius.sm).stroke(LognColors.line, lineWidth: 1))
    }

    /// Verde e vermelho só aparecem como veredito ou **ação destrutiva irreversível**,
    /// e nesse caso só dentro do bloco de confirmação — nunca no repouso da tela.
    private var deletionBlock: some View {
        VStack(alignment: .leading, spacing: 0) {
            Text(Str.Logout.grace_period)
                .font(.plexMono(10.5))
                .tracking(0.14 * 10.5)
                .foregroundColor(LognColors.wrongInk)

            Text(Str.Logout.delete_account)
                .font(.plexSansSemiBold(15, relativeTo: .callout))
                .foregroundColor(LognColors.textPrimary)
                .padding(.top, 8)

            Text(Str.Logout.delete_desc(Int(core.viewModel.globalXp)))
                .font(.plexSans(13, relativeTo: .footnote))
                .lineSpacing(19 - 13)
                .foregroundColor(LognColors.textSecondary)
                .fixedSize(horizontal: false, vertical: true)
                .padding(.top, 7)

            Button(action: { showingDeleteConfirm = true }) {
                Text(Str.Logout.delete_button)
                    .font(.plexSansSemiBold(14.5))
                    .foregroundColor(LognColors.wrongInk)
                    .frame(maxWidth: .infinity, minHeight: 46)
                    .overlay(RoundedRectangle(cornerRadius: Radius.sm).stroke(LognColors.wrong, lineWidth: 1))
            }
            .padding(.top, 14)

            Text(Str.Logout.delete_prompt)
                .font(.plexMono(10.5))
                .foregroundColor(LognColors.textMuted)
                .frame(maxWidth: .infinity)
                .padding(.top, 9)
        }
        .padding(16)
        .background(LognColors.tintErr)
        .cornerRadius(Radius.sm)
        .overlay(RoundedRectangle(cornerRadius: Radius.sm).stroke(LognColors.wrong, lineWidth: 1))
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
    /// O GitHub não emite ID token: o Core troca o código pelo bilhete e pelo token com
    /// que o servidor revoga, e só então pede a exclusão (ADR 0019).
    let onDeleteWithGitHub: (GitHubAuth.Credential) -> Void
    @Environment(\.dismiss) private var dismiss
    @Environment(\.colorScheme) private var colorScheme
    @EnvironmentObject var core: CoreWrapper

    @State private var password = ""
    @State private var confirmation = ""
    @State private var providerBusy = false

    var body: some View {
        VStack(alignment: .leading, spacing: 20) {
            Text(Str.Logout.delete_account)
                .font(.plexSansSemiBold(18))
                .foregroundColor(LognColors.textPrimary)

            Text(Str.Logout.delete_desc(Int(core.viewModel.globalXp)))
                .font(.plexSans(14))
                .foregroundColor(LognColors.textSecondary)
                
            Text(Str.Logout.grace_period)
                .font(.plexSansSemiBold(14))
                .foregroundColor(LognColors.wrongInk)

            PasswordField(Str.Login.password_prompt, text: $password)
                .font(.plexMono(14))
                .padding(.horizontal, 14)
                .frame(height: 52)
                .background(LognColors.surface)
                .cornerRadius(Radius.sm)
                .overlay(RoundedRectangle(cornerRadius: Radius.sm).stroke(LognColors.line, lineWidth: 1))
                .foregroundColor(LognColors.textPrimary)

            TextField(Str.Logout.delete_confirm_placeholder, text: $confirmation,
                      prompt: lightPrompt(Str.Logout.delete_confirm_placeholder, colorScheme))
                .font(.plexMono(14))
                .padding(.horizontal, 14)
                .frame(height: 52)
                .background(LognColors.surface)
                .cornerRadius(Radius.sm)
                .overlay(RoundedRectangle(cornerRadius: Radius.sm).stroke(LognColors.line, lineWidth: 1))
                .foregroundColor(LognColors.textPrimary)

            Button(action: {
                if canDelete {
                    // Sobra de uma exclusão pelo Google que falhou não é revogada por esta.
                    GoogleAuth.shared.pendingRevocation = ""
                    onDelete(password)
                    dismiss()
                }
            }) {
                // A tinta sobre o `wrong` cheio, que escurece no claro.
                Text(Str.Logout.delete_button)
                    .font(.plexSansSemiBold(15))
                    .foregroundColor(canDelete ? LognColors.onStatus : LognColors.textDim)
                    .frame(maxWidth: .infinity, minHeight: 52)
                    .background(canDelete ? LognColors.wrong : LognColors.buttonDisabled)
                    .cornerRadius(Radius.sm)
            }
            .disabled(!canDelete)

            if AppleAuth.shared.isConfigured {
                providerButton(Str.Logout.delete_with_apple, action: deleteWithApple)
            }
            if GoogleAuth.shared.isConfigured {
                providerButton(Str.Logout.delete_with_google, action: deleteWithGoogle)
            }
            if GitHubAuth.shared.isConfigured {
                providerButton(Str.Logout.delete_with_github, action: deleteWithGitHub)
            }
        }
        .padding(20)
        // A folha inteira, não só o conteúdo: sem o quadro cheio, o fundo do sistema
        // aparecia em cima e embaixo, e o conteúdo lia como um cartão dentro da folha.
        .frame(maxWidth: .infinity, maxHeight: .infinity)
        .background(LognColors.surfaceRaised.ignoresSafeArea())
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
                .foregroundColor(confirmed ? LognColors.textSecondary : LognColors.textDim)
                .frame(maxWidth: .infinity, minHeight: 44)
        }
        // Desligado, o texto já sai em `textDim`; o `.plain` ainda esmaecia pela metade
        // por cima, e o rótulo sumia (1.7:1 no claro).
        .buttonStyle(PressSinkStyle())
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

    private func deleteWithGitHub() {
        providerBusy = true
        Task {
            defer { providerBusy = false }
            do {
                let credential = try await GitHubAuth.shared.signIn()
                GoogleAuth.shared.pendingRevocation = ""
                // O servidor troca o código e revoga no GitHub; o aparelho não faz nada.
                onDeleteWithGitHub(credential)
                dismiss()
            } catch GitHubAuth.Failure.cancelled {
                // Desistiu na janela do GitHub: a conta fica.
            } catch {
                core.dispatch(event: .socialLoginFailed)
            }
        }
    }
}
