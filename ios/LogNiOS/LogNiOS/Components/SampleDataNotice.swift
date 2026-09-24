import SwiftUI
import LogNCoreFFI
import App

/// Tarja de "dados de exemplo", no topo do ranking e do telão.
///
/// Os dois mostram times, posições e um "você em 42º" que o jogador lia como fato: o
/// placar ainda não tem servidor e os números saem do mock do design system. Em `info`,
/// não em `warn`: nada está quebrado, é só algo que ainda não está no ar.
struct SampleDataNotice: View {
    var body: some View {
        HStack(alignment: .top, spacing: Space.sm) {
            Image(systemName: "testtube.2")
                .font(.system(size: 12, weight: .semibold))
                .padding(.top, 1)
            Text(Str.Status.sample_standings)
                .font(.plexSans(12.5))
                .fixedSize(horizontal: false, vertical: true)
            Spacer(minLength: 0)
        }
        .foregroundColor(LognDark.infoInk)
        .padding(.horizontal, 20)
        .padding(.vertical, 10)
        .frame(maxWidth: .infinity, alignment: .leading)
        .background(LognDark.tintInfo)
        .overlay(alignment: .bottom) {
            Rectangle().frame(height: 1).foregroundColor(LognDark.info)
        }
        .accessibilityElement(children: .combine)
    }
}
