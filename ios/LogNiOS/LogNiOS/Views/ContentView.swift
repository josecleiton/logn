import SwiftUI
import App

struct ContentView: View {
    @EnvironmentObject var core: CoreWrapper
    
    var body: some View {
        NavigationView {
            ZStack {
                LognDark.canvas.ignoresSafeArea()
                
                VStack(spacing: Space.xl) {
                    // Cabecalho
                    Text("LogN App")
                        .font(LognFont.headlineMedium)
                        .foregroundColor(LognDark.textPrimary)
                    
                    // Status Box
                    VStack(spacing: Space.md) {
                        Text(core.viewModel.displayStatus)
                            .font(LognFont.bodyLarge)
                            .foregroundColor(LognDark.info)
                            .multilineTextAlignment(.center)
                        
                        HStack {
                            Image(systemName: "arrow.triangle.2.circlepath")
                                .foregroundColor(core.viewModel.pendingSyncCount > 0 ? LognDark.warn : LognDark.textMuted)
                            Text("\(core.viewModel.pendingSyncCount) pending syncs")
                                .font(LognFont.label)
                                .foregroundColor(LognDark.textSecondary)
                            
                            if core.viewModel.isSyncing || core.viewModel.isFetching {
                                ProgressView()
                                    .progressViewStyle(CircularProgressViewStyle(tint: LognDark.accent))
                                    .padding(.leading, Space.sm)
                            }
                        }
                    }
                    .padding(Space.lg)
                    .background(LognDark.surfaceRaised)
                    .cornerRadius(Radius.md)
                    .overlay(
                        RoundedRectangle(cornerRadius: Radius.md)
                            .stroke(LognDark.line, lineWidth: 1)
                    )
                    
                    // Botoes Crux
                    VStack(spacing: Space.md) {
                        Button(action: {
                            core.dispatch(event: .syncNow)
                        }) {
                            Text("Sync Now (HTTP POST)")
                                .font(LognFont.titleMedium)
                                .frame(maxWidth: .infinity)
                                .padding(.vertical, Space.md)
                                .background(core.viewModel.isSyncing ? LognDark.buttonDisabled : LognDark.accent)
                                .foregroundColor(core.viewModel.isSyncing ? LognDark.textMuted : LognDark.onAccent)
                                .cornerRadius(Radius.sm)
                        }
                        .disabled(core.viewModel.isSyncing)
                        
                        Button(action: {
                            core.dispatch(event: .fetchChallenges)
                        }) {
                            Text("Load Challenges (HTTP GET)")
                                .font(LognFont.titleMedium)
                                .frame(maxWidth: .infinity)
                                .padding(.vertical, Space.md)
                                .background(core.viewModel.isFetching ? LognDark.buttonDisabled : LognDark.info)
                                .foregroundColor(core.viewModel.isFetching ? LognDark.textMuted : LognDark.surface)
                                .cornerRadius(Radius.sm)
                        }
                        .disabled(core.viewModel.isFetching)
                    }
                    .padding(.horizontal, Space.screenMargin)
                    
                    // Lista Dinamica
                    ScrollView {
                        VStack(spacing: Space.md) {
                            ForEach(core.viewModel.challenges, id: \.id) { challenge in
                                if challenge.templateType == "SPOT_THE_BUG" {
                                    NavigationLink(destination: SpotTheBugView(
                                        title: challenge.payload.content.title,
                                        codeLines: challenge.payload.content.codeLines,
                                        balloonColor: Balloon.of(challenge.payload.content.title.first ?? "A", isLight: false),
                                        onSubmit: { line in
                                            let payload = "{\"selected_line\": \(line)}"
                                            let timestamp = Int64(Date().timeIntervalSince1970)
                                            core.dispatch(event: .submitChallengeAnswer(actionId: UUID().uuidString, challengeId: challenge.id, answerJson: payload, timestamp: timestamp))
                                        }
                                    )) {
                                        ChallengeRow(challenge: challenge)
                                    }
                                } else if challenge.templateType == "FILL_IN_THE_BLANK" {
                                    NavigationLink(destination: FillInTheBlankView(
                                        title: challenge.payload.content.title,
                                        codeLines: challenge.payload.content.codeLines,
                                        balloonColor: Balloon.of(challenge.payload.content.title.first ?? "A", isLight: false),
                                        onSubmit: { answer in
                                            let payload = "{\"answer_string\": \"\(answer)\"}"
                                            let timestamp = Int64(Date().timeIntervalSince1970)
                                            core.dispatch(event: .submitChallengeAnswer(actionId: UUID().uuidString, challengeId: challenge.id, answerJson: payload, timestamp: timestamp))
                                        }
                                    )) {
                                        ChallengeRow(challenge: challenge)
                                    }
                                } else {
                                    // Fallback UI for other types
                                    ChallengeRow(challenge: challenge)
                                }
                            }
                        }
                        .padding(.horizontal, Space.screenMargin)
                    }
                }
                .padding(.top)
            }
            .colorScheme(.dark)
        }
    }
}

struct ChallengeRow: View {
    let challenge: Challenge
    
    var body: some View {
        HStack {
            VStack(alignment: .leading, spacing: Space.xs) {
                Text(challenge.payload.content.title)
                    .font(LognFont.titleMedium)
                    .foregroundColor(LognDark.textPrimary)
                Text(challenge.templateType)
                    .font(LognFont.label)
                    .foregroundColor(LognDark.textSecondary)
            }
            Spacer()
            Image(systemName: "chevron.right")
                .foregroundColor(LognDark.lineDim)
        }
        .padding(Space.md)
        .background(LognDark.surface)
        .cornerRadius(Radius.md)
        .overlay(
            RoundedRectangle(cornerRadius: Radius.md)
                .stroke(LognDark.rowLine, lineWidth: 1)
        )
    }
}
