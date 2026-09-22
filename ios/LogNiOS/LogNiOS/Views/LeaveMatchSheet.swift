import SwiftUI
import LogN

/// Confirmação de saída da partida.
///
/// O Duolingo avisa que sair perde o progresso. Aqui isso seria mentira: o XP entra por
/// resposta aceita e já foi para a fila de sync, e o MATCH_END não credita nada. Então o
/// cartão não ameaça com uma perda que não acontece — ele diz o que de fato acaba, que é
/// a partida, e mostra os balões que ficam para trás.
///
/// Quem decide se este cartão aparece é o Core: sem balão no ar e com as três vidas, o X
/// sai direto. Pergunta que aparece quando não há o que perguntar ensina o jogador a
/// dispensá-la sem ler.
struct LeaveMatchSheet: View {
    let solved: Int
    let total: Int
    /// (letra, aceito) dos problemas do nó, para desenhar o que fica.
    let balloonStates: [(Character, Bool)]
    let onStay: () -> Void
    let onLeave: () -> Void

    var body: some View {
        VStack(alignment: .leading, spacing: 0) {
            Text(Str.Leave.eyebrow.uppercased())
                .font(.plexMono(11))
                .tracking(0.12 * 11)
                .foregroundColor(LognDark.textMuted)

            Text(Str.Leave.solved(solved, total))
                .font(.plexSansSemiBold(26, relativeTo: .title2))
                .foregroundColor(LognDark.textPrimary)
                .fixedSize(horizontal: false, vertical: true)
                .padding(.top, 10)

            Text(Str.Leave.body)
                .font(.plexSans(16, relativeTo: .body))
                .lineSpacing(16 * 0.45)
                .foregroundColor(LognDark.textSecondary)
                .fixedSize(horizontal: false, vertical: true)
                .padding(.top, 14)

            // O que fica para trás, desenhado: perda concreta em vez de adjetivo.
            if !pendentes.isEmpty {
                HStack(spacing: 10) {
                    ForEach(pendentes, id: \.self) { letra in
                        VStack(spacing: 4) {
                            BalloonShape(style: .outline(LognDark.lineStrong, 4), width: 14, showString: false)
                            Text(String(letra))
                                .font(.plexMono(9))
                                .foregroundColor(LognDark.textMuted)
                        }
                    }
                    Spacer(minLength: 0)
                }
                .padding(.top, 18)
            }

            // Ficar é o caminho fácil: cheio, embaixo, onde o polegar está. Quem tocou no
            // X por engano acerta o caminho de volta sem mirar.
            Button(action: onStay) {
                Text(Str.Leave.stay)
                    .font(.plexSansSemiBold(16, relativeTo: .body))
                    .foregroundColor(LognDark.onAccent)
                    .frame(maxWidth: .infinity)
                    .padding(.vertical, 16)
                    .background(LognDark.accent)
                    .cornerRadius(Radius.sm)
            }
            .padding(.top, 28)

            Button(action: onLeave) {
                Text(Str.Leave.confirm)
                    .font(.plexSansMedium(16, relativeTo: .body))
                    .foregroundColor(LognDark.textSecondary)
                    .frame(maxWidth: .infinity)
                    .padding(.vertical, 16)
                    .overlay(
                        RoundedRectangle(cornerRadius: Radius.sm)
                            .stroke(LognDark.lineStrong, lineWidth: 1)
                    )
            }
            .padding(.top, 10)
        }
        .padding(.horizontal, 24)
        .padding(.top, 44)
        .padding(.bottom, 24)
        .frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .topLeading)
        .background(LognDark.surface)
    }

    /// As letras que ainda não subiram.
    private var pendentes: [Character] {
        balloonStates.filter { !$0.1 }.map { $0.0 }
    }
}
