import SwiftUI
import LogNCoreFFI
import LogN

/// A tela que cobre o app quando há versão relevante dos termos para aceitar (ADR 0020,
/// design `LogN Splash`, tela 02). O Core decide quando ela aparece e o que ela lista; a
/// caixa é daqui: nunca vem marcada, e o botão só aceita com ela marcada.
struct TermsUpdateView: View {
    @EnvironmentObject var core: CoreWrapper
    let update: LogN.TermsUpdateViewModel

    @State private var checked = false
    @State private var showsDocuments = false
    @State private var showsDecline = false

    var body: some View {
        VStack(spacing: 0) {
            ScrollView {
                VStack(alignment: .leading, spacing: Space.lg) {
                    Text(Str.Reaccept.eyebrow)
                        .font(.plexMono(11))
                        .tracking(0.14 * 11)
                        .foregroundColor(LognColors.infoInk)
                        .padding(.top, Space.sm)

                    Text(headline)
                        .font(.plexSansSemiBold(23, relativeTo: .title2))
                        .foregroundColor(LognColors.textPrimary)
                        .fixedSize(horizontal: false, vertical: true)

                    versions
                    changesList

                    Button { showsDocuments = true } label: {
                        Text(Str.Reaccept.read_documents)
                            .font(.plexSans(14))
                            .underline()
                            .foregroundColor(LognColors.textSecondary)
                            .frame(minHeight: Space.minTouch)
                    }
                    .buttonStyle(.plain)
                }
                .padding(.horizontal, 22)
                .padding(.bottom, Space.lg)
            }

            footer
        }
        .background(LognColors.canvas.ignoresSafeArea())
        .sheet(isPresented: $showsDocuments) {
            // Abre nos termos, com as seções novas marcadas; a troca para a política fica
            // no cabeçalho do documento. O destaque vale só para o primeiro documento, e
            // os dois têm seções de mesmo id (`exclusao`): as da política não vão juntas.
            LegalDocumentView(kind: .terms, highlight: update.termsSections)
        }
        .sheet(isPresented: $showsDecline) {
            TermsDeclineSheet().environmentObject(core)
        }
    }

    /// O que a tela mostra, como identidade: versão e mudanças. Mudou, a tela nasce de
    /// novo, com a caixa desmarcada.
    static func identity(of update: LogN.TermsUpdateViewModel) -> String {
        var parts = ["\(update.fromVersion)", "\(update.toVersion)"]
        for change in update.changes {
            parts.append("\(change.kind):\(change.version):\(change.section)")
        }
        return parts.joined(separator: "|")
    }

    private var headline: String {
        if update.fromVersion == 0 { return Str.Reaccept.headline_first }
        if update.versionsSkipped > 1 { return Str.Reaccept.headline_times(Int(update.versionsSkipped)) }
        return Str.Reaccept.headline
    }

    // MARK: - Versões

    private var versions: some View {
        HStack(spacing: 0) {
            versionCell(
                label: Str.Reaccept.you_accepted,
                version: update.fromVersion == 0 ? Str.Reaccept.never_accepted : Str.Reaccept.version(Int(update.fromVersion)),
                date: update.fromDate,
                labelColor: LognColors.textMuted,
                valueColor: LognColors.textSecondary
            )
            Image(systemName: "arrow.right")
                .font(.system(size: 13, weight: .semibold))
                .foregroundColor(LognColors.textMuted)
                .frame(width: 20)
                .accessibilityHidden(true)
            versionCell(
                label: Str.Reaccept.current,
                version: Str.Reaccept.version(Int(update.toVersion)),
                date: update.toDate,
                labelColor: LognColors.infoInk,
                valueColor: LognColors.textPrimary
            )
            .overlay(alignment: .leading) {
                Rectangle().fill(LognColors.line).frame(width: 1)
            }
        }
        .background(LognColors.surface)
        .cornerRadius(Radius.sm)
        .overlay(RoundedRectangle(cornerRadius: Radius.sm).stroke(LognColors.line, lineWidth: 1))
    }

    private func versionCell(label: String, version: String, date: String, labelColor: Color, valueColor: Color) -> some View {
        VStack(alignment: .leading, spacing: 3) {
            Text(label)
                .font(.plexMono(9.5))
                .tracking(0.12 * 9.5)
                .foregroundColor(labelColor)
            Text(version)
                .font(.plexMono(17))
                .foregroundColor(valueColor)
            Text(date)
                .font(.plexMono(10.5))
                .foregroundColor(LognColors.textMuted)
        }
        .frame(maxWidth: .infinity, alignment: .leading)
        .padding(.horizontal, 12)
        .padding(.vertical, 11)
        .accessibilityElement(children: .combine)
    }

    // MARK: - O que mudou

    private var changesList: some View {
        VStack(alignment: .leading, spacing: Space.sm) {
            HStack {
                Text(Str.Reaccept.what_changed)
                Spacer()
                Text(Str.Reaccept.changes_count(update.changes.count))
            }
            .font(.plexMono(10))
            .tracking(0.12 * 10)
            .foregroundColor(LognColors.textMuted)

            ForEach(Array(update.changes.enumerated()), id: \.offset) { _, change in
                TermsChangeRow(change: change)
            }
        }
    }

    // MARK: - Rodapé

    private var footer: some View {
        VStack(spacing: Space.md) {
            Button { checked.toggle() } label: {
                HStack(alignment: .top, spacing: 10) {
                    RoundedRectangle(cornerRadius: Radius.xs + 1)
                        .stroke(checked ? LognColors.accent : LognColors.lineDim, lineWidth: 1.5)
                        .background(RoundedRectangle(cornerRadius: Radius.xs + 1).fill(checked ? LognColors.accent : Color.clear))
                        .frame(width: 20, height: 20)
                        .overlay {
                            Image(systemName: "checkmark")
                                .font(.system(size: 11, weight: .bold))
                                .foregroundColor(LognColors.onAccent)
                                .opacity(checked ? 1 : 0)
                        }
                    Text(Str.Reaccept.accept_check(Int(update.toVersion)))
                        .font(.plexSans(13))
                        .foregroundColor(LognColors.textSecondary)
                        .multilineTextAlignment(.leading)
                        .fixedSize(horizontal: false, vertical: true)
                    Spacer(minLength: 0)
                }
                .frame(minHeight: Space.minTouch)
                .contentShape(Rectangle())
            }
            .buttonStyle(.plain)
            .accessibilityAddTraits(checked ? [.isSelected] : [])

            Button { core.dispatch(event: .acceptTerms) } label: {
                Text(update.accepting ? Str.Reaccept.accepting : Str.Reaccept.accept_button)
                    .font(.plexSansSemiBold(16))
                    .foregroundColor(canAccept ? LognColors.onAccent : LognColors.textDim)
                    .frame(maxWidth: .infinity, minHeight: 52)
                    .background(canAccept ? LognColors.accent : LognColors.buttonDisabled)
                    .cornerRadius(Radius.sm)
            }
            .disabled(!canAccept)

            Button { showsDecline = true } label: {
                Text(Str.Reaccept.decline)
                    .font(.plexSans(14))
                    .foregroundColor(LognColors.textSecondary)
                    .frame(maxWidth: .infinity, minHeight: Space.minTouch)
            }
            .buttonStyle(.plain)
            .disabled(update.accepting)
        }
        .padding(.horizontal, 22)
        .padding(.top, 14)
        .padding(.bottom, 12)
        .background(LognColors.surface.ignoresSafeArea(edges: .bottom))
        .overlay(alignment: .top) {
            Rectangle().fill(LognColors.line).frame(height: 1)
        }
    }

    private var canAccept: Bool { checked && !update.accepting }
}

/// Uma mudança do diff: o sinal numa faixa tingida, de onde veio e a frase.
private struct TermsChangeRow: View {
    let change: LogN.TermsChange

    var body: some View {
        HStack(alignment: .top, spacing: 0) {
            Text(sign)
                .font(.plexMono(14))
                .foregroundColor(ink)
                .frame(width: 22)
                .padding(.top, 8)
                .frame(maxHeight: .infinity, alignment: .top)
                .background(tint)
                .overlay(alignment: .trailing) {
                    Rectangle().fill(line).frame(width: 2)
                }
            VStack(alignment: .leading, spacing: 3) {
                Text(place)
                    .font(.plexMono(9.5))
                    .tracking(0.08 * 9.5)
                    .foregroundColor(LognColors.textMuted)
                Text(change.summary)
                    .font(.plexSans(13))
                    .foregroundColor(LognColors.textPrimary)
                    .fixedSize(horizontal: false, vertical: true)
            }
            .padding(.horizontal, 10)
            .padding(.vertical, 8)
            Spacer(minLength: 0)
        }
        .fixedSize(horizontal: false, vertical: true)
        .background(LognColors.surface)
        .cornerRadius(Radius.xs + 1)
        .overlay(RoundedRectangle(cornerRadius: Radius.xs + 1).stroke(LognColors.line, lineWidth: 1))
        .accessibilityElement(children: .ignore)
        .accessibilityLabel(Str.Reaccept.change_accessibility(signName, place, change.summary))
    }

    private var place: String {
        let document = change.kind == "privacy" ? Str.Reaccept.doc_privacy : Str.Reaccept.doc_terms
        return Str.Reaccept.change_where(document, Int(change.version))
    }

    private var sign: String {
        switch change.change {
        case .added: return "+"
        case .changed: return "~"
        case .removed: return "−"
        }
    }

    private var signName: String {
        switch change.change {
        case .added: return Str.Reaccept.sign_added
        case .changed: return Str.Reaccept.sign_changed
        case .removed: return Str.Reaccept.sign_removed
        }
    }

    /// O sinal (+, ~, −) é glifo: a tinta. A barra ao lado é traço: a base.
    private var ink: Color {
        switch change.change {
        case .added: return LognColors.correctInk
        case .changed: return LognColors.warnInk
        case .removed: return LognColors.wrongInk
        }
    }

    private var line: Color {
        switch change.change {
        case .added: return LognColors.correct
        case .changed: return LognColors.warn
        case .removed: return LognColors.wrong
        }
    }

    private var tint: Color {
        switch change.change {
        case .added: return LognColors.tintOk
        case .changed: return LognColors.tintWarn
        case .removed: return LognColors.tintErr
        }
    }
}

/// "Não concordo": quem está bloqueado não alcança o Perfil, então a saída e a exclusão
/// ficam aqui (design, "Registro do aceite").
struct TermsDeclineSheet: View {
    @EnvironmentObject var core: CoreWrapper
    @Environment(\.dismiss) private var dismiss
    @State private var showsDelete = false

    var body: some View {
        // O fundo cobre a folha inteira, como no `CriticalLogoutSheet`: com ele só atrás do
        // conteúdo, sobrava a cor padrão da folha em cima e embaixo.
        ZStack(alignment: .top) {
            LognColors.surfaceRaised.ignoresSafeArea()
            content
        }
        .presentationDetents([.medium])
        .sheet(isPresented: $showsDelete) {
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

    private var content: some View {
        VStack(alignment: .leading, spacing: Space.lg) {
            Text(Str.Reaccept.decline_title)
                .font(.plexSansSemiBold(18))
                .foregroundColor(LognColors.textPrimary)
            Text(Str.Reaccept.decline_body)
                .font(.plexSans(14))
                .foregroundColor(LognColors.textSecondary)
                .fixedSize(horizontal: false, vertical: true)

            Button {
                dismiss()
                core.dispatch(event: .logout)
            } label: {
                // A borda do botão secundário (`lineStrong`, como no `LognButton`): com
                // `line` o botão quase sumia na folha.
                Text(Str.Reaccept.sign_out)
                    .font(.plexSansSemiBold(15))
                    .foregroundColor(LognColors.textPrimary)
                    .frame(maxWidth: .infinity, minHeight: 50)
                    .overlay(RoundedRectangle(cornerRadius: Radius.sm).stroke(LognColors.lineStrong, lineWidth: 1))
            }

            Button { showsDelete = true } label: {
                Text(Str.Reaccept.delete_account)
                    .font(.plexSansSemiBold(15))
                    .foregroundColor(LognColors.wrongInk)
                    .frame(maxWidth: .infinity, minHeight: 50)
                    .overlay(RoundedRectangle(cornerRadius: Radius.sm).stroke(LognColors.wrong, lineWidth: 1))
            }

            Button { dismiss() } label: {
                Text(Str.Reaccept.back)
                    .font(.plexSans(14))
                    .foregroundColor(LognColors.textSecondary)
                    .frame(maxWidth: .infinity, minHeight: Space.minTouch)
            }
            .buttonStyle(.plain)
        }
        .padding(.horizontal, 20)
        .padding(.top, 28)
    }
}

/// A faixa das mudanças não relevantes, uma vez, na árvore. O aceite já foi gravado
/// quando ela apareceu; fechar só tira a faixa.
struct TermsNoticeBanner: View {
    @EnvironmentObject var core: CoreWrapper
    @State private var showsDocument = false

    var body: some View {
        HStack(alignment: .center, spacing: Space.md) {
            Text(Str.Reaccept.notice)
                .font(.plexSans(13))
                .foregroundColor(LognColors.textPrimary)
                .fixedSize(horizontal: false, vertical: true)
                .padding(.vertical, 12)
            Spacer(minLength: 0)
            Button { showsDocument = true } label: {
                Text(Str.Reaccept.notice_open)
                    .font(.plexSansMedium(13))
                    .foregroundColor(LognColors.infoInk)
                    .frame(minWidth: Space.minTouch, minHeight: Space.minTouch)
            }
            .buttonStyle(.plain)
            Button { core.dispatch(event: .dismissTermsNotice) } label: {
                Image(systemName: "xmark")
                    .font(.system(size: 12, weight: .semibold))
                    .foregroundColor(LognColors.textMuted)
                    .frame(width: Space.minTouch, height: Space.minTouch)
            }
            .buttonStyle(.plain)
            .accessibilityLabel(Str.Reaccept.notice_close)
        }
        .padding(.leading, 14)
        .background(LognColors.tintInfo)
        .cornerRadius(Radius.sm)
        .overlay(RoundedRectangle(cornerRadius: Radius.sm).stroke(LognColors.info, lineWidth: 1))
        .padding(.horizontal, Space.screenMargin)
        .sheet(isPresented: $showsDocument) {
            LegalDocumentView(kind: .terms)
        }
    }
}
