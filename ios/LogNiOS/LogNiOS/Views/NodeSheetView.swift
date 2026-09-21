import SwiftUI
import LogN

struct NodeSheetView: View {
    let node: SkillNode
    let onStartMatch: () -> Void
    
    @Environment(\.dismiss) var dismiss
    
    var body: some View {
        ZStack {
            LognDark.surface.ignoresSafeArea()
            
            VStack(spacing: Space.xl) {
                // Header Image/Icon
                Circle()
                    .fill(LognDark.surfaceRaised)
                    .frame(width: 80, height: 80)
                    .overlay(
                        Image(systemName: "cube.transparent.fill")
                            .font(.system(size: 32))
                            .foregroundColor(node.status == .completed ? LognDark.info : LognDark.accent)
                    )
                    .padding(.top, Space.xl)
                
                // Content
                VStack(spacing: Space.sm) {
                    Text(node.name)
                        .font(LognFont.headlineMedium)
                        .foregroundColor(LognDark.textPrimary)
                        .multilineTextAlignment(.center)
                    
                    Text(node.description)
                        .font(LognFont.bodyLarge)
                        .foregroundColor(LognDark.textSecondary)
                        .multilineTextAlignment(.center)
                        .padding(.horizontal, Space.lg)
                }
                
                // Node Stats
                HStack(spacing: Space.xl) {
                    VStack {
                        Text("XP MÍNIMO")
                            .font(.custom("IBMPlexMono-Medium", size: 10))
                            .foregroundColor(LognDark.textMuted)
                        Text("\(node.requiredXp)")
                            .font(LognFont.titleMedium)
                            .foregroundColor(LognDark.textPrimary)
                    }
                    
                    VStack {
                        Text("STATUS")
                            .font(.custom("IBMPlexMono-Medium", size: 10))
                            .foregroundColor(LognDark.textMuted)
                        
                        Text(node.status == .completed ? "CONCLUÍDO" : "ATIVO")
                            .font(LognFont.titleMedium)
                            .foregroundColor(node.status == .completed ? LognDark.info : LognDark.accent)
                    }
                }
                .padding(.vertical, Space.md)
                .frame(maxWidth: .infinity)
                .background(LognDark.surfaceRaised)
                .cornerRadius(Radius.md)
                .padding(.horizontal, Space.screenMargin)
                
                Spacer()
                
                // Start Button
                LognButton(title: "JOGAR AGORA", variant: .primary) {
                    dismiss()
                    // Small delay to allow sheet to dismiss before pushing navigation
                    DispatchQueue.main.asyncAfter(deadline: .now() + 0.3) {
                        onStartMatch()
                    }
                }
                .padding(.horizontal, Space.screenMargin)
                .padding(.bottom, Space.xl)
            }
        }
    }
}
