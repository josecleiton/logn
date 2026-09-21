import SwiftUI
import LogN
import App

struct StandingsView: View {
    @EnvironmentObject var core: CoreWrapper
    @State private var selectedTab = 0 // 0 = Global, 1 = Sede
    
    // Mocks for now until the Rust engine exposes the Standings API
    let mockGlobal = [
        StandingRow(rank: 1, name: "GennadyK", solved: 13, penalty: 345, isCurrentUser: false),
        StandingRow(rank: 2, name: "tourist", solved: 13, penalty: 412, isCurrentUser: false),
        StandingRow(rank: 3, name: "Petr", solved: 12, penalty: 981, isCurrentUser: false),
        StandingRow(rank: 4, name: "Um_nik", solved: 12, penalty: 1044, isCurrentUser: false),
        StandingRow(rank: 341, name: "Você (Guest)", solved: 4, penalty: 210, isCurrentUser: true)
    ]
    
    let mockSede = [
        StandingRow(rank: 1, name: "Você (Guest)", solved: 4, penalty: 210, isCurrentUser: true),
        StandingRow(rank: 2, name: "Joãozinho", solved: 3, penalty: 150, isCurrentUser: false),
        StandingRow(rank: 3, name: "Maria", solved: 3, penalty: 320, isCurrentUser: false)
    ]
    
    var body: some View {
        ZStack {
            LognDark.canvas.ignoresSafeArea()
            
            VStack(spacing: 0) {
                // Header
                VStack(spacing: Space.md) {
                    Text("PLACAR GERAL")
                        .font(LognFont.headlineMedium)
                        .foregroundColor(LognDark.textPrimary)
                    
                    // Custom Segmented Control
                    HStack(spacing: 0) {
                        TabButton(title: "GLOBAL", isSelected: selectedTab == 0) { selectedTab = 0 }
                        TabButton(title: "SEDE", isSelected: selectedTab == 1) { selectedTab = 1 }
                    }
                    .background(LognDark.surfaceRaised)
                    .cornerRadius(Radius.md)
                    .padding(.horizontal, Space.screenMargin)
                }
                .padding(.vertical, Space.lg)
                .background(LognDark.surface)
                
                // Header da Tabela
                HStack {
                    Text("#")
                        .font(LognFont.label)
                        .foregroundColor(LognDark.textMuted)
                        .frame(width: 30, alignment: .leading)
                    
                    Text("COMPETIDOR")
                        .font(LognFont.label)
                        .foregroundColor(LognDark.textMuted)
                        
                    Spacer()
                    
                    Text("AC")
                        .font(LognFont.label)
                        .foregroundColor(LognDark.textMuted)
                        .frame(width: 30, alignment: .trailing)
                    
                    Text("PEN")
                        .font(LognFont.label)
                        .foregroundColor(LognDark.textMuted)
                        .frame(width: 40, alignment: .trailing)
                }
                .padding(.horizontal, Space.screenMargin)
                .padding(.vertical, Space.sm)
                .background(LognDark.surfaceRaised)
                
                // Lista de Rankings
                ScrollView {
                    VStack(spacing: 0) {
                        let activeList = selectedTab == 0 ? mockGlobal : mockSede
                        ForEach(activeList, id: \.name) { row in
                            StandingRowView(row: row)
                            Divider().background(LognDark.line)
                        }
                    }
                    
                    NavigationLink(destination: ScoreboardView()) {
                        Text("VER TELÃO COMPLETO (ICPC)")
                            .font(LognFont.label)
                            .frame(maxWidth: .infinity)
                            .padding(.vertical, Space.md)
                            .background(LognDark.surfaceRaised)
                            .foregroundColor(LognDark.accent)
                            .cornerRadius(Radius.sm)
                    }
                    .padding(Space.lg)
                }
            }
        }
    }
}

struct TabButton: View {
    let title: String
    let isSelected: Bool
    let action: () -> Void
    
    var body: some View {
        Button(action: action) {
            Text(title)
                .font(LognFont.label)
                .frame(maxWidth: .infinity)
                .padding(.vertical, 12)
                .background(isSelected ? LognDark.surface : Color.clear)
                .foregroundColor(isSelected ? LognDark.textPrimary : LognDark.textSecondary)
                .cornerRadius(Radius.sm)
                .overlay(
                    RoundedRectangle(cornerRadius: Radius.sm)
                        .stroke(isSelected ? LognDark.line : Color.clear, lineWidth: 1)
                )
                .padding(2)
        }
    }
}

struct StandingRow {
    let rank: Int
    let name: String
    let solved: Int
    let penalty: Int
    let isCurrentUser: Bool
}

struct StandingRowView: View {
    let row: StandingRow
    
    var body: some View {
        HStack {
            Text("\(row.rank)")
                .font(.custom("IBMPlexMono-Medium", size: 14))
                .foregroundColor(row.isCurrentUser ? LognDark.onAccent : LognDark.textSecondary)
                .frame(width: 30, alignment: .leading)
            
            Text(row.name)
                .font(LognFont.titleMedium)
                .foregroundColor(row.isCurrentUser ? LognDark.onAccent : LognDark.textPrimary)
                .lineLimit(1)
            
            Spacer()
            
            Text("\(row.solved)")
                .font(.custom("IBMPlexMono-Medium", size: 14))
                .foregroundColor(row.isCurrentUser ? LognDark.onAccent : LognDark.info)
                .frame(width: 30, alignment: .trailing)
            
            Text("\(row.penalty)")
                .font(.custom("IBMPlexMono-Regular", size: 14))
                .foregroundColor(row.isCurrentUser ? LognDark.onAccent.opacity(0.8) : LognDark.textMuted)
                .frame(width: 40, alignment: .trailing)
        }
        .padding(.horizontal, Space.screenMargin)
        .padding(.vertical, Space.md)
        .background(row.isCurrentUser ? LognDark.accent : Color.clear)
    }
}
