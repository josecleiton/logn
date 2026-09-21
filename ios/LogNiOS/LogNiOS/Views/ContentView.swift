import SwiftUI
import App

struct ContentView: View {
    @StateObject private var core = CoreWrapper()
    
    var body: some View {
        ZStack {
            LognDark.canvas.ignoresSafeArea()
            
            VStack(spacing: Space.xl) {
                // Cabecalho usando as cores e fontes do Design System
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
                
                // Botoes para gerar eventos do Crux
                VStack(spacing: Space.md) {
                    Button(action: {
                        core.dispatch(event: .ping)
                    }) {
                        Text("Send Ping")
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
                    
                    Button(action: {
                        let timestamp = Int64(Date().timeIntervalSince1970)
                        core.dispatch(event: .registerAction(actionId: UUID().uuidString, actionType: "SOLVE", payloadJson: "{\"correct\": true}", timestamp: timestamp))
                    }) {
                        Text("Register Action (Mini-Git)")
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
                }
                .padding(.horizontal, Space.screenMargin)
            }
            .padding()
        }
        // Configurando default color scheme para Dark no exemplo
        .colorScheme(.dark)
    }
}
