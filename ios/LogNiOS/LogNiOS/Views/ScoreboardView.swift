import SwiftUI
import LogN

struct ScoreboardView: View {
    @Environment(\.dismiss) var dismiss
    
    // 13 problemas de A a M
    let problems = (0..<13).map { String(Character(UnicodeScalar(65 + $0)!)) }
    
    // Mocks super complexos
    let mockTeams = [
        ScoreboardRow(rank: 1, name: "GennadyK", solved: 13, penalty: 345, isUser: false, verdicts: ["AC","AC","AC","AC","AC","AC","AC","AC","AC","AC","AC","AC","AC"]),
        ScoreboardRow(rank: 2, name: "tourist", solved: 12, penalty: 412, isUser: false, verdicts: ["AC","AC","AC","AC","WA","AC","AC","AC","AC","AC","AC","AC","AC"]),
        ScoreboardRow(rank: 3, name: "Petr", solved: 12, penalty: 981, isUser: false, verdicts: ["AC","AC","AC","AC","AC","WA","AC","WA","AC","AC","AC","AC","AC"]),
        ScoreboardRow(rank: 124, name: "Você (Guest)", solved: 4, penalty: 210, isUser: true, verdicts: ["AC","WA","AC","","","","AC","","","","","AC",""])
    ]
    
    var body: some View {
        ZStack {
            LognDark.canvas.ignoresSafeArea()
            
            VStack(spacing: 0) {
                // Header
                HStack {
                    Button(action: { dismiss() }) {
                        Image(systemName: "xmark")
                            .font(.system(size: 20, weight: .bold))
                            .foregroundColor(LognDark.textSecondary)
                    }
                    Spacer()
                    Text("SCOREBOARD ICPC")
                        .font(LognFont.label)
                        .foregroundColor(LognDark.textMuted)
                    Spacer()
                    Image(systemName: "xmark").opacity(0)
                }
                .padding(.horizontal, Space.screenMargin)
                .padding(.vertical, Space.md)
                .background(LognDark.surfaceRaised)
                
                // Tabela Scrollável Bidirecional
                ScrollView([.horizontal, .vertical]) {
                    VStack(alignment: .leading, spacing: 0) {
                        
                        // Cabeçalho da Tabela
                        HStack(spacing: 0) {
                            Text("#")
                                .frame(width: 40, alignment: .center)
                                .font(LognFont.label)
                                .foregroundColor(LognDark.textMuted)
                            
                            Text("COMPETIDOR")
                                .frame(width: 150, alignment: .leading)
                                .font(LognFont.label)
                                .foregroundColor(LognDark.textMuted)
                                .padding(.leading, Space.sm)
                            
                            Text("AC/PEN")
                                .frame(width: 70, alignment: .center)
                                .font(LognFont.label)
                                .foregroundColor(LognDark.textMuted)
                            
                            ForEach(problems, id: \.self) { p in
                                Text(p)
                                    .frame(width: 44, alignment: .center)
                                    .font(LognFont.label)
                                    .foregroundColor(LognDark.textMuted)
                            }
                        }
                        .frame(height: 40)
                        .background(LognDark.surface)
                        
                        // Linhas
                        ForEach(mockTeams, id: \.name) { team in
                            HStack(spacing: 0) {
                                Text("\(team.rank)")
                                    .frame(width: 40, alignment: .center)
                                    .font(.plexMonoMedium(14))
                                    .foregroundColor(team.isUser ? LognDark.onAccent : LognDark.textSecondary)
                                
                                Text(team.name)
                                    .frame(width: 150, alignment: .leading)
                                    .font(LognFont.titleMedium)
                                    .foregroundColor(team.isUser ? LognDark.onAccent : LognDark.textPrimary)
                                    .lineLimit(1)
                                    .padding(.leading, Space.sm)
                                
                                VStack(spacing: 2) {
                                    Text("\(team.solved)")
                                        .font(.plexMonoMedium(14))
                                        .foregroundColor(team.isUser ? LognDark.onAccent : LognDark.info)
                                    Text("\(team.penalty)")
                                        .font(.plexMono(10))
                                        .foregroundColor(team.isUser ? LognDark.onAccent.opacity(0.8) : LognDark.textMuted)
                                }
                                .frame(width: 70, alignment: .center)
                                
                                ForEach(0..<13, id: \.self) { i in
                                    let v = team.verdicts[i]
                                    ZStack {
                                        if v == "AC" {
                                            Rectangle().fill(LognDark.correct).opacity(0.15)
                                            Text("+")
                                                .font(.plexMonoMedium(14))
                                                .foregroundColor(LognDark.correct)
                                        } else if v == "WA" {
                                            Rectangle().fill(LognDark.wrong).opacity(0.15)
                                            Text("-1")
                                                .font(.plexMonoMedium(14))
                                                .foregroundColor(LognDark.wrong)
                                        } else {
                                            Text("")
                                        }
                                    }
                                    .frame(width: 44, height: 44)
                                    .border(LognDark.line, width: 0.5)
                                }
                            }
                            .frame(height: 44)
                            .background(team.isUser ? LognDark.accent.opacity(0.8) : Color.clear)
                            .border(LognDark.line, width: 0.5)
                        }
                    }
                    .padding(Space.md)
                }
            }
        }
        .navigationBarHidden(true)
    }
}

struct ScoreboardRow {
    let rank: Int
    let name: String
    let solved: Int
    let penalty: Int
    let isUser: Bool
    let verdicts: [String]
}
