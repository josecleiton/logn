import SwiftUI
import App

struct ContentView: View {
    @StateObject private var core = CoreWrapper()
    @State private var showingGame = false
    
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
                            
                            if core.viewModel.isSyncing {
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
                        
                        NavigationLink(destination: SpotTheBugView(
                            title: "C · Soma de Dois Números",
                            codeLines: [
                                "int l = 0, r = n - 1;",
                                "while (l <= r) {",
                                "    int mid = l + (r - l) / 2;",
                                "    if (a == b) return a;",
                                "    if (a < b) a = a + 1;",
                                "    else b = b - 1;",
                                "}",
                                "return -1;"
                            ],
                            balloonColor: Balloon.of("C", isLight: false),
                            onSubmit: { line in
                                let payload = "{\"selected_line\": \(line)}"
                                let timestamp = Int64(Date().timeIntervalSince1970)
                                core.dispatch(event: .registerAction(actionId: UUID().uuidString, actionType: "SPOT_THE_BUG", payloadJson: payload, timestamp: timestamp))
                            }
                        )) {
                            Text("Play 'Spot the Bug'")
                                .font(LognFont.titleMedium)
                                .frame(maxWidth: .infinity)
                                .padding(.vertical, Space.md)
                                .background(LognDark.surface)
                                .foregroundColor(LognDark.textPrimary)
                                .cornerRadius(Radius.sm)
                                .overlay(
                                    RoundedRectangle(cornerRadius: Radius.sm)
                                        .stroke(LognDark.lineStrong, lineWidth: 1)
                                )
                        }
                    }
                    .padding(.horizontal, Space.screenMargin)
                }
                .padding()
            }
            .colorScheme(.dark)
        }
    }
}
