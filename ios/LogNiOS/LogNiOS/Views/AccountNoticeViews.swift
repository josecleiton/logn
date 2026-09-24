import SwiftUI
import LogNCoreFFI
import LogN

/// Depois de pedir a exclusão: a conta está desativada, e diz até quando entrar ainda
/// a traz de volta. Mesmo molde da despedida do logout, sem desfazer — a sessão já foi
/// recusada pelo servidor, e o caminho de volta é entrar de novo.
struct DeletionNoticeView: View {
    @EnvironmentObject var core: CoreWrapper

    var body: some View {
        ZStack {
            LognDark.canvas.ignoresSafeArea()

            VStack(spacing: 0) {
                VStack(spacing: 16) {
                    Image(systemName: "clock.arrow.circlepath")
                        .font(.system(size: 34, weight: .light))
                        .foregroundColor(LognDark.textMuted)
                        .accessibilityHidden(true)

                    Text(Str.Logout.deleted_title)
                        .font(.plexSansSemiBold(18, relativeTo: .headline))
                        .foregroundColor(LognDark.textPrimary)
                        .multilineTextAlignment(.center)

                    Text(Str.Logout.deleted_body(purgeDate))
                        .font(.plexSans(14, relativeTo: .subheadline))
                        .lineSpacing(21 - 14)
                        .foregroundColor(LognDark.textSecondary)
                        .multilineTextAlignment(.center)
                        .fixedSize(horizontal: false, vertical: true)
                }
                .frame(maxHeight: .infinity)
                .padding(.horizontal, 34)

                Button {
                    core.dispatch(event: .dismissDeletionNotice)
                } label: {
                    Text(Str.Logout.notice_ok)
                        .font(.plexSansMedium(15))
                        .foregroundColor(LognDark.textPrimary)
                        .frame(maxWidth: .infinity, minHeight: 50)
                        .overlay(RoundedRectangle(cornerRadius: Radius.sm).stroke(LognDark.lineStrong, lineWidth: 1))
                }
                .buttonStyle(PressSinkStyle())
                .padding(.horizontal, 20)
                .padding(.bottom, 28)
            }
        }
    }

    /// A data do expurgo por extenso, na língua do app.
    private var purgeDate: String {
        let date = Date(timeIntervalSince1970: TimeInterval(core.viewModel.deletionPurgeAfter))
        return date.formatted(Date.FormatStyle(date: .long, time: .omitted).locale(Locale(identifier: AppLocale.current)))
    }
}

/// Entrou dentro da carência: a exclusão foi cancelada. Aparece uma vez, por cima do
/// jogo, e some com o toque.
struct AccountRestoredCard: View {
    @EnvironmentObject var core: CoreWrapper

    var body: some View {
        VStack(alignment: .leading, spacing: 0) {
            Text(Str.Logout.restored_title)
                .font(.plexSansSemiBold(15, relativeTo: .callout))
                .foregroundColor(LognDark.textPrimary)

            Text(Str.Logout.restored_body)
                .font(.plexSans(13.5, relativeTo: .footnote))
                .lineSpacing(20 - 13.5)
                .foregroundColor(LognDark.textSecondary)
                .fixedSize(horizontal: false, vertical: true)
                .padding(.top, 6)

            Button {
                core.dispatch(event: .dismissAccountRestoredNotice)
            } label: {
                Text(Str.Logout.notice_ok)
                    .font(.plexSansMedium(14.5))
                    .foregroundColor(LognDark.textPrimary)
                    .frame(maxWidth: .infinity, minHeight: 44)
                    .overlay(RoundedRectangle(cornerRadius: Radius.sm).stroke(LognDark.lineStrong, lineWidth: 1))
            }
            .buttonStyle(PressSinkStyle())
            .padding(.top, 14)
        }
        .padding(16)
        .background(LognDark.surfaceRaised)
        .cornerRadius(Radius.md)
        .overlay(RoundedRectangle(cornerRadius: Radius.md).stroke(LognDark.lineStrong, lineWidth: 1))
        .padding(.horizontal, 20)
        .padding(.bottom, 24)
        .accessibilityElement(children: .contain)
    }
}
