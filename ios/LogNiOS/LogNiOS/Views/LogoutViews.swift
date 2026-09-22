import SwiftUI
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

                    Text("Seus \(vm.globalXp) XP estão no servidor. Entrar de novo traz tudo de volta.")
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
                        Text("Entrar de novo")
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
                            Text("saiu por engano? desfazer")
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
        vm.displayName.isEmpty ? "Até a próxima" : "Até a próxima, \(vm.displayName)"
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
                Text("\(pendingCount) EVENTOS NA FILA · \(xpAtRisk) XP")
                    .font(.plexMono(10.5))
                    .tracking(0.14 * 10.5)
                    .foregroundColor(LognDark.warnInk)

                Text("Vamos salvar antes de sair")
                    .font(.plexSansSemiBold(18, relativeTo: .headline))
                    .lineSpacing(18 * 0.3)
                    .foregroundColor(LognDark.textPrimary)
                    .padding(.top, 9)

                Text("Esse progresso ainda não chegou ao servidor. Sincronizar leva alguns segundos e você não perde nada.")
                    .font(.plexSans(14, relativeTo: .subheadline))
                    .lineSpacing(21 - 14)
                    .foregroundColor(LognDark.textSecondary)
                    .fixedSize(horizontal: false, vertical: true)
                    .padding(.top, 9)

                Button(action: onSyncAndLeave) {
                    Text("Sincronizar e sair")
                        .font(.plexSansSemiBold(15, relativeTo: .callout))
                        .foregroundColor(LognDark.onAccent)
                        .frame(maxWidth: .infinity, minHeight: 50)
                        .background(LognDark.accent)
                        .cornerRadius(Radius.sm)
                }
                .buttonStyle(PressSinkStyle())
                .padding(.top, 18)

                Button(action: onStay) {
                    Text("Continuar conectado")
                        .font(.plexSansMedium(15))
                        .foregroundColor(LognDark.textPrimary)
                        .frame(maxWidth: .infinity, minHeight: 48)
                        .overlay(RoundedRectangle(cornerRadius: Radius.sm).stroke(LognDark.lineStrong, lineWidth: 1))
                }
                .buttonStyle(PressSinkStyle())
                .padding(.top, 8)

                // Texto, não botão: a perda deixa de ser o caminho padrão.
                Button(action: onDiscard) {
                    Text("Sair e descartar \(xpAtRisk) XP")
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

    var body: some View {
        ZStack {
            LognDark.canvas.ignoresSafeArea()

            VStack(alignment: .leading, spacing: 0) {
                header

                VStack(spacing: 10) {
                    actionRow("Trocar e-mail", sub: core.viewModel.otpEmail)
                    actionRow("Trocar senha", sub: nil)
                    actionRow("Baixar meus dados", sub: "JSON com XP, trilhas e submissões")
                }
                .padding(.horizontal, 20)
                .padding(.top, 18)

                Spacer(minLength: 18)

                deletionBlock
                    .padding(.horizontal, 20)
                    .padding(.bottom, 20)
            }
        }
        .navigationBarHidden(true)
    }

    private var header: some View {
        HStack(spacing: 14) {
            Button { dismiss() } label: {
                Image(systemName: "chevron.left")
                    .font(.system(size: 16, weight: .semibold))
                    .foregroundColor(LognDark.textSecondary)
            }
            .accessibilityLabel("Voltar")

            Text("Gerenciar conta")
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
            Text("IRREVERSÍVEL")
                .font(.plexMono(10.5))
                .tracking(0.14 * 10.5)
                .foregroundColor(LognDark.wrongInk)

            Text("Excluir conta")
                .font(.plexSansSemiBold(15, relativeTo: .callout))
                .foregroundColor(LognDark.textPrimary)
                .padding(.top, 8)

            Text("Apaga o e-mail, os \(core.viewModel.globalXp) XP, as trilhas e o histórico de submissões do servidor. Nada disso volta.")
                .font(.plexSans(13, relativeTo: .footnote))
                .lineSpacing(19 - 13)
                .foregroundColor(LognDark.textSecondary)
                .fixedSize(horizontal: false, vertical: true)
                .padding(.top, 7)

            Text("Excluir minha conta")
                .font(.plexSansSemiBold(14.5))
                .foregroundColor(LognDark.wrongInk)
                .frame(maxWidth: .infinity, minHeight: 46)
                .overlay(RoundedRectangle(cornerRadius: Radius.sm).stroke(LognDark.wrong, lineWidth: 1))
                .padding(.top, 14)

            Text("pede a senha e a palavra EXCLUIR")
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
