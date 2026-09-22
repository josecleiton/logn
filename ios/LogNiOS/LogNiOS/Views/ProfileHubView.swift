import SwiftUI
import LogN
import App

struct ProfileHubView: View {
    @EnvironmentObject var core: CoreWrapper
    @Environment(\.dismiss) var dismiss
    @State private var showLogoutWarning = false
    @State private var showSettings = false
    
    var body: some View {
        ZStack {
            LognDark.canvas.ignoresSafeArea()
            
            VStack(spacing: 0) {
                // Navegação customizada
                HStack {
                    Button(action: { dismiss() }) {
                        Image(systemName: "xmark")
                            .font(.system(size: 20, weight: .bold))
                            .foregroundColor(LognDark.textSecondary)
                    }
                    Spacer()
                    Text("PERFIL")
                        .font(LognFont.label)
                        .foregroundColor(LognDark.textMuted)
                    Spacer()
                    Button(action: {
                        showSettings = true
                    }) {
                        Image(systemName: "gearshape.fill")
                            .font(.system(size: 20))
                            .foregroundColor(LognDark.textSecondary)
                    }
                }
                .padding(.horizontal, Space.screenMargin)
                .padding(.vertical, Space.md)
                
                ScrollView {
                    VStack(spacing: Space.xl) {
                        
                        // User Info Card
                        VStack(spacing: Space.md) {
                            Circle()
                                .fill(LognDark.surface)
                                .frame(width: 80, height: 80)
                                .overlay(
                                    Image(systemName: "person.crop.circle.fill")
                                        .resizable()
                                        .foregroundColor(LognDark.textMuted)
                                )
                            
                            VStack(spacing: 4) {
                                Text(core.viewModel.isGuest ? "Visitante" : core.viewModel.otpEmail)
                                    .font(LognFont.titleMedium)
                                    .foregroundColor(LognDark.textPrimary)
                                
                                if core.viewModel.isGuest {
                                    Text("Progresso salvo apenas no dispositivo")
                                        .font(LognFont.bodyLarge)
                                        .foregroundColor(LognDark.warnInk)
                                }
                            }
                        }
                        .padding(.top, Space.lg)
                        
                        // XP e Nível
                        let xp = Int(core.viewModel.globalXp)
                        let level = (xp / 200) + 1
                        let xpInLevel = xp % 200
                        let progress = Double(xpInLevel) / 200.0
                        
                        VStack(alignment: .leading, spacing: Space.sm) {
                            HStack {
                                Text("NÍVEL \(level)")
                                    .font(LognFont.label)
                                    .foregroundColor(LognDark.accentInk)
                                Spacer()
                                Text("\(xpInLevel) / 200 XP")
                                    .font(LognFont.label)
                                    .foregroundColor(LognDark.textSecondary)
                            }
                            
                            GeometryReader { geo in
                                ZStack(alignment: .leading) {
                                    Capsule()
                                        .fill(LognDark.surfaceRaised)
                                        .frame(height: 8)
                                    
                                    Capsule()
                                        .fill(LognDark.accent)
                                        .frame(width: geo.size.width * CGFloat(progress), height: 8)
                                }
                            }
                            .frame(height: 8)
                        }
                        .padding(.horizontal, Space.screenMargin)
                        
                        // Stats Grid
                        HStack(spacing: Space.md) {
                            StatBox(title: "XP TOTAL", value: "\(xp)")
                            StatBox(title: "BUGS", value: "0") // TODO: ViewModels stats
                            StatBox(title: "DRY RUNS", value: "\(core.viewModel.dryRunsCompleted)")
                        }
                        .padding(.horizontal, Space.screenMargin)
                        
                        // Sync Status
                        VStack(alignment: .leading, spacing: Space.sm) {
                            Text("SINCRONIZAÇÃO")
                                .font(LognFont.label)
                                .foregroundColor(LognDark.textMuted)
                                .padding(.horizontal, Space.screenMargin)
                            
                            HStack {
                                Circle()
                                    .fill(core.viewModel.pendingSyncCount > 0 ? LognDark.warn : LognDark.correct)
                                    .frame(width: 8, height: 8)
                                
                                Text(core.viewModel.pendingSyncCount > 0 ? "\(core.viewModel.pendingSyncCount) edições locais pendentes" : "Tudo sincronizado na nuvem")
                                    .font(LognFont.bodyLarge)
                                    .foregroundColor(LognDark.textPrimary)
                                
                                Spacer()
                                
                                if core.viewModel.isSyncing {
                                    ProgressView().progressViewStyle(CircularProgressViewStyle(tint: LognDark.accent))
                                } else {
                                    Button("Sincronizar") {
                                        core.dispatch(event: .syncNow)
                                    }
                                    .font(LognFont.label)
                                    .foregroundColor(LognDark.accentInk)
                                }
                            }
                            .padding(Space.md)
                            .background(LognDark.surface)
                            .cornerRadius(Radius.sm)
                            .overlay(RoundedRectangle(cornerRadius: Radius.sm).stroke(LognDark.line, lineWidth: 1))
                            .padding(.horizontal, Space.screenMargin)
                        }
                        
                        if core.viewModel.isGuest {
                            LognButton(title: "Criar Conta Gratuita", variant: .primary) {
                                // Envia para logout/login
                                core.dispatch(event: .logout)
                            }
                            .padding(.horizontal, Space.screenMargin)
                        } else {
                            LognButton(title: "Sair da Conta", variant: .ghost) {
                                if core.viewModel.pendingSyncCount > 0 {
                                    showLogoutWarning = true
                                } else {
                                    core.dispatch(event: .logout)
                                }
                            }
                            .padding(.horizontal, Space.screenMargin)
                        }
                        
                        Spacer().frame(height: 100)
                    }
                }
            }
        }
        .navigationBarHidden(true)
        .alert(isPresented: $showLogoutWarning) {
            Alert(
                title: Text("Atenção!"),
                message: Text("Você possui \(core.viewModel.pendingSyncCount) edições locais que ainda não foram enviadas. Se você sair agora, perderá esse progresso.\n\nRecomendamos clicar em Sincronizar antes de sair."),
                primaryButton: .destructive(Text("Sair e perder progresso")) {
                    core.dispatch(event: .logout)
                },
                secondaryButton: .cancel(Text("Cancelar"))
            )
        }
        .fullScreenCover(isPresented: $showSettings) {
            SettingsView().environmentObject(core)
        }
    }
}

struct StatBox: View {
    let title: String
    let value: String
    var body: some View {
        VStack(alignment: .leading, spacing: 4) {
            Text(title)
                .font(.plexMonoMedium(10))
                .foregroundColor(LognDark.textMuted)
            Text(value)
                .font(LognFont.titleMedium)
                .foregroundColor(LognDark.textPrimary)
        }
        .frame(maxWidth: .infinity, alignment: .leading)
        .padding(Space.md)
        .background(LognDark.surface)
        .cornerRadius(Radius.sm)
        .overlay(RoundedRectangle(cornerRadius: Radius.sm).stroke(LognDark.line, lineWidth: 1))
    }
}
