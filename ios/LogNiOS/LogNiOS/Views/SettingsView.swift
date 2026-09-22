import SwiftUI
import LogN
import App

struct SettingsView: View {
    @EnvironmentObject var core: CoreWrapper
    @Environment(\.dismiss) var dismiss
    
    // UI State gerido puramente pelo SwiftUI
    @AppStorage("hapticsEnabled") private var hapticsEnabled = true
    @AppStorage("notificationsEnabled") private var notificationsEnabled = false
    
    @State private var showDeleteWarning = false
    
    var body: some View {
        ZStack {
            LognDark.canvas.ignoresSafeArea()
            
            VStack(spacing: 0) {
                // NavBar
                HStack {
                    Button(action: { dismiss() }) {
                        Image(systemName: "chevron.left")
                            .font(.system(size: 20, weight: .bold))
                            .foregroundColor(LognDark.textSecondary)
                    }
                    Spacer()
                    Text("GERENCIAR CONTA")
                        .lognLabel()
                        .foregroundColor(LognDark.textMuted)
                    Spacer()
                    // Espaçador para equilibrar o chevron
                    Image(systemName: "chevron.left").opacity(0)
                }
                .padding(.horizontal, Space.screenMargin)
                .padding(.vertical, Space.md)
                
                ScrollView {
                    VStack(spacing: Space.xl) {
                        
                        // Section: Preferências de App
                        VStack(alignment: .leading, spacing: Space.md) {
                            Text("PREFERÊNCIAS DO APP")
                                .lognLabel()
                                .foregroundColor(LognDark.textMuted)
                                .padding(.horizontal, Space.screenMargin)
                            
                            VStack(spacing: 0) {
                                // O interruptor de tema saiu: `LognLight` existe nos tokens,
                                // mas nenhum mock desenha o app em claro, e um controle que
                                // só mudava a status bar é pior que controle nenhum.
                                ToggleRow(title: "Feedback Tátil (Haptics)", isOn: $hapticsEnabled)
                                Divider().background(LognDark.line)
                                ToggleRow(title: "Notificações Push", isOn: $notificationsEnabled)
                            }
                            .background(LognDark.surface)
                            .cornerRadius(Radius.sm)
                            .overlay(RoundedRectangle(cornerRadius: Radius.sm).stroke(LognDark.line, lineWidth: 1))
                            .padding(.horizontal, Space.screenMargin)
                        }
                        .padding(.top, Space.lg)
                        
                        // Section: Conta
                        if !core.viewModel.isGuest {
                            VStack(alignment: .leading, spacing: Space.md) {
                                Text("SUA CONTA")
                                    .lognLabel()
                                    .foregroundColor(LognDark.textMuted)
                                    .padding(.horizontal, Space.screenMargin)
                                
                                VStack(spacing: 0) {
                                    Button(action: {
                                        // TODO: Navegar para Reset Password ou Mudar Senha
                                        core.dispatch(event: .requestOtp(email: core.viewModel.accountEmail, purpose: "reset_password"))
                                    }) {
                                        HStack {
                                            Text("Alterar Senha")
                                                .font(LognFont.bodyLarge)
                                                .foregroundColor(LognDark.textPrimary)
                                            Spacer()
                                            Image(systemName: "chevron.right")
                                                .foregroundColor(LognDark.textMuted)
                                        }
                                        .padding(Space.md)
                                    }
                                    
                                    Divider().background(LognDark.line)
                                    
                                    Button(action: {
                                        // TODO: Fluxo de LGPD
                                    }) {
                                        HStack {
                                            Text("Exportar Dados")
                                                .font(LognFont.bodyLarge)
                                                .foregroundColor(LognDark.textPrimary)
                                            Spacer()
                                            Image(systemName: "square.and.arrow.up")
                                                .foregroundColor(LognDark.textMuted)
                                        }
                                        .padding(Space.md)
                                    }
                                }
                                .background(LognDark.surface)
                                .cornerRadius(Radius.sm)
                                .overlay(RoundedRectangle(cornerRadius: Radius.sm).stroke(LognDark.line, lineWidth: 1))
                                .padding(.horizontal, Space.screenMargin)
                            }
                        }
                        
                        // Section: Zona de Perigo
                        VStack(alignment: .leading, spacing: Space.md) {
                            Text("ZONA DE PERIGO")
                                .lognLabel()
                                .foregroundColor(LognDark.warnInk)
                                .padding(.horizontal, Space.screenMargin)
                            
                            Button(action: {
                                showDeleteWarning = true
                            }) {
                                HStack {
                                    Text("Excluir Conta")
                                        .font(LognFont.bodyLarge)
                                        .foregroundColor(LognDark.warnInk)
                                    Spacer()
                                    Image(systemName: "trash")
                                        .foregroundColor(LognDark.warnInk)
                                }
                                .padding(Space.md)
                                .background(LognDark.surface)
                                .cornerRadius(Radius.sm)
                                .overlay(RoundedRectangle(cornerRadius: Radius.sm).stroke(LognDark.warn.opacity(0.3), lineWidth: 1))
                                .padding(.horizontal, Space.screenMargin)
                            }
                        }
                    }
                }
            }
        }
        .navigationBarHidden(true)
        .alert(isPresented: $showDeleteWarning) {
            Alert(
                title: Text("Excluir Conta Permanentemente?"),
                message: Text("Esta ação não pode ser desfeita. Todo o seu XP, histórico e dados sincronizados serão apagados para sempre."),
                primaryButton: .destructive(Text("Sim, excluir conta")) {
                    // TODO: Dispatch Delete Account
                    core.dispatch(event: .logout) // Temporário até ter endpoint
                },
                secondaryButton: .cancel(Text("Cancelar"))
            )
        }
    }
}

struct ToggleRow: View {
    let title: String
    @Binding var isOn: Bool
    
    var body: some View {
        Toggle(isOn: $isOn) {
            Text(title)
                .font(LognFont.bodyLarge)
                .foregroundColor(LognDark.textPrimary)
        }
        .tint(LognDark.accent)
        .padding(Space.md)
    }
}
