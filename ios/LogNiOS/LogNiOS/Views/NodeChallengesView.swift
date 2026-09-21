import SwiftUI
import LogN
import App

struct NodeChallengesView: View {
    @EnvironmentObject var core: CoreWrapper
    let node: SkillNode
    
    var body: some View {
        let nodeChallenges = core.viewModel.challenges.filter { $0.nodeId == node.id }
        
        ZStack {
            LognDark.canvas.ignoresSafeArea()
            
            if nodeChallenges.isEmpty {
                Text("No challenges available for this node yet.")
                    .font(LognFont.bodyLarge)
                    .foregroundColor(LognDark.textSecondary)
                    .padding()
            } else {
                ScrollView {
                    VStack(spacing: Space.md) {
                        ForEach(nodeChallenges, id: \.id) { challenge in
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
                                ChallengeRow(challenge: challenge)
                            }
                        }
                    }
                    .padding()
                }
            }
        }
        .navigationTitle(node.name)
        .navigationBarTitleDisplayMode(.inline)
    }
}
