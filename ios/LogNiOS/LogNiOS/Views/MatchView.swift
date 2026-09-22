import SwiftUI
import LogN
import App
import UniformTypeIdentifiers

/// Partida — exploração `3b · Contest`.
///
/// O placar fica sempre à vista: problema atual, relógio, vidas e a fileira A—M no topo.
/// O veredito toma a tela inteira, como no telão do ginásio.
struct MatchView: View {
    @EnvironmentObject var core: CoreWrapper
    let nodeId: String
    @Environment(\.dismiss) var dismiss
    @Environment(\.accessibilityReduceMotion) private var reduceMotion

    /// Veredito congelado no instante do submit — o Core já avançou para o próximo
    /// problema quando esta tela aparece, então a letra e a sigla precisam ser guardadas.
    @State private var verdict: VerdictSnapshot?
    @State private var timer: Timer?

    struct VerdictSnapshot {
        let letter: Character
        let code: VerdictChip.Verdict
        let livesLeft: Int
        var isAccepted: Bool { code == .ac }
    }

    private var mv: LogN.MatchViewModel { core.viewModel.matchView }

    private var currentLetter: Character { mv.currentLetter.first ?? "A" }

    private var balloonStates: [(Character, Bool)] {
        mv.balloonStates.compactMap { state in
            state.letter.first.map { ($0, state.isAccepted) }
        }
    }

    var body: some View {
        ZStack {
            LognDark.canvas.ignoresSafeArea()

            if let verdict {
                MatchVerdictScreen(
                    snapshot: verdict,
                    reduceMotion: reduceMotion,
                    balloonStates: balloonStates,
                    remainingSeconds: Int(mv.questionSeconds),
                    lives: Int(mv.lives),
                    maxLives: Int(mv.maxLives),
                    trapCategory: mv.trapCategory,
                    trapTitle: mv.trapTitle,
                    trapExplanation: mv.trapExplanation,
                    matchOver: !mv.isActive,
                    xpAward: 50,
                    onContinue: dismissVerdict
                )
                .transition(.opacity)
            } else if !mv.isActive && !core.viewModel.matchLeft {
                // Relatório é para quem jogou até o fim. Quem saiu pelo X não tem o que
                // revisar, e o `onChange` abaixo já está levando a tela de volta.
                MatchReportView(onDismiss: { dismiss() })
            } else {
                playScreen
            }
        }
        .navigationBarHidden(true)
        .onAppear {
            core.dispatch(event: .startMatch(nodeId: nodeId))
            startTimer()
        }
        // Quem decide que a partida acabou é o Core; empilhar ou desempilhar tela é do
        // shell. Ele diz que a saída foi por vontade do jogador, e a tela volta.
        .onChange(of: core.viewModel.matchLeft) { saiu in
            if saiu { dismiss() }
        }
        .onChange(of: mv.hasTrap) { hasTrap in
            // O TLE não vem de um toque: o relógio zera e o Core submete sozinho.
            // Sem isto a trap ficaria aberta sem tela e o relógio travaria.
            guard hasTrap, verdict == nil else { return }
            withAnimation(.easeOut(duration: 0.12)) {
                verdict = VerdictSnapshot(
                    letter: currentLetter,
                    code: VerdictChip.Verdict(code: mv.lastVerdict),
                    livesLeft: Int(mv.lives)
                )
            }
        }
        .onDisappear {
            timer?.invalidate()
            timer = nil
        }
    }

    // MARK: - Tela de jogo

    private var playScreen: some View {
        VStack(spacing: 0) {
            MatchHeader(
                letter: currentLetter,
                remainingSeconds: Int(mv.questionSeconds),
                isFrozen: mv.isFrozen,
                lives: Int(mv.lives),
                maxLives: Int(mv.maxLives),
                balloonStates: balloonStates,
                onLeave: { core.dispatch(event: .leaveMatch) }
            )

            if fillsHeight {
                // SPOT_THE_BUG cabe na tela, e o documento dá `flex:1` ao bloco de
                // código: a moldura vai até o CTA. Num ScrollView isso não acontece,
                // porque o filho recebe altura ilimitada e nunca estica.
                VStack(alignment: .leading, spacing: 0) {
                    problemStatement
                    templateBody
                        .padding(.horizontal, 14)
                        .padding(.top, 16)
                        .padding(.bottom, 16)
                        .frame(maxHeight: .infinity)
                }
            } else {
                ScrollView {
                    VStack(alignment: .leading, spacing: 0) {
                        problemStatement
                        templateBody
                            .padding(.horizontal, 14)
                            .padding(.top, 16)
                            .padding(.bottom, 24)
                    }
                }
            }

            // CTA ancorado — nomeia a escolha, para que um toque errado seja reversível.
            LognButton(
                title: buttonTitle,
                variant: .primary,
                action: submit,
                isDisabled: !canSubmit
            )
            .padding(.horizontal, 14)
            .padding(.top, 16)
            .padding(.bottom, 22)
            .background(LognDark.canvas)
        }
        // Quem manda no cartão é o Core: ele decide se esta leitura para o relógio, e
        // arrastar para fechar precisa avisá-lo para o relógio voltar.
        .sheet(isPresented: Binding(
            get: { !mv.originSheet.isEmpty },
            set: { aberto in if !aberto { core.dispatch(event: .closeOriginSheet) } }
        )) {
            OriginSheetView(
                origin: mv.originSheet,
                clockPaused: mv.originSheetPaused,
                onClose: { core.dispatch(event: .closeOriginSheet) }
            )
            .presentationDetents([.large])
            .modifier(SheetCorners())
        }
        // Arrastar para baixo é o mesmo que ficar: quem some com o cartão sem escolher
        // está voltando para a partida, e o relógio tem de voltar junto.
        .sheet(isPresented: Binding(
            get: { mv.leavePending },
            set: { aberto in if !aberto { core.dispatch(event: .cancelLeaveMatch) } }
        )) {
            LeaveMatchSheet(
                solved: Int(mv.solvedSoFar),
                total: Int(mv.totalProblems),
                balloonStates: balloonStates,
                onStay: { core.dispatch(event: .cancelLeaveMatch) },
                onLeave: { core.dispatch(event: .confirmLeaveMatch) }
            )
            .presentationDetents([.medium, .large])
            .modifier(SheetCorners())
        }
    }

    /// Só SPOT_THE_BUG estica: o documento dá `flex:1` ao bloco de código dele.
    private var fillsHeight: Bool { mv.currentTemplateType == "SPOT_THE_BUG" }

    /// Nome do problema em `label` + o enunciado em 19sp/600.
    private var problemStatement: some View {
        VStack(alignment: .leading, spacing: 0) {
            HStack(spacing: 8) {
                Text(mv.currentTitle.uppercased())
                    .font(.plexMono(11))
                    .tracking(0.12 * 11)
                    .foregroundColor(LognDark.textMuted)

                // Nem todo desafio nasceu aqui. Quando veio de fora, a origem fica na
                // linha do título — atribuição que ninguém vê não é atribuição — e o
                // selo abre a história de quem escreveu o problema.
                if !mv.currentOrigin.isEmpty {
                    Button {
                        core.dispatch(event: .openOriginSheet)
                    } label: {
                        Text(mv.currentOrigin.uppercased())
                            .font(.plexMono(10))
                            .tracking(0.12 * 10)
                            .foregroundColor(LognDark.accent)
                            .padding(.horizontal, 6)
                            .padding(.vertical, 2)
                            .overlay(
                                RoundedRectangle(cornerRadius: Radius.sm)
                                    .stroke(LognDark.accent.opacity(0.5), lineWidth: 1)
                            )
                    }
                    .buttonStyle(.plain)
                }

                Spacer(minLength: 0)
            }

            Text(mv.currentDescription)
                .font(.plexSansSemiBold(19, relativeTo: .title3))
                .lineSpacing(19 * 0.3)
                .foregroundColor(LognDark.textPrimary)
                .fixedSize(horizontal: false, vertical: true)
                .padding(.top, 12)
        }
        .frame(maxWidth: .infinity, alignment: .leading)
        .padding(.horizontal, 18)
        .padding(.top, 22)
    }

    @ViewBuilder
    private var templateBody: some View {
        switch mv.currentTemplateType {
        case "SPOT_THE_BUG":
            CodeBlock(
                lines: mv.currentCodeLines,
                selectedLine: mv.selectedLine >= 0 ? Int(mv.selectedLine) : nil,
                highlightColor: LognDark.accent,
                fillsHeight: true,
                onSelectLine: { core.dispatch(event: .matchSelectLine(line: Int32($0))) }
            )

        case "DRY_RUN":
            DryRunPanel(
                codeLines: mv.currentCodeLines,
                watchVariables: mv.watchVariables,
                watchNote: mv.watchNote,
                predictedOutput: mv.predictedOutput,
                onOutputChange: { core.dispatch(event: .matchSetOutput(value: $0)) }
            )

        case "FILL_IN_THE_BLANK":
            VStack(alignment: .leading, spacing: 16) {
                CodeBlock(lines: mv.currentCodeLines)

                Text("ARRASTE O BLOCO")
                    .font(.plexMono(11))
                    .tracking(0.12 * 11)
                    .foregroundColor(LognDark.textMuted)

                DropZone(
                    value: mv.answerString,
                    onDrop: { core.dispatch(event: .matchSetAnswer(answer: $0)) },
                    onRemove: { core.dispatch(event: .matchSetAnswer(answer: "")) }
                )

                chipBank(mv.currentOptions.filter { $0 != mv.answerString })
            }

        case "COMPLEXITY_MATCH":
            VStack(alignment: .leading, spacing: 10) {
                // Estes dois templates viviam só de prosa: a rotina era descrita no
                // enunciado e o jogador tinha de imaginá-la. Quando o desafio traz
                // código, mostrar vale mais que descrever — e evita a discussão sobre
                // qual é o tipo da variável, que é onde a complexidade se decide.
                if !mv.currentCodeLines.isEmpty {
                    CodeBlock(lines: mv.currentCodeLines)
                }

                labelledDrop(
                    "TEMPO",
                    value: mv.dropTime,
                    onDrop: { core.dispatch(event: .matchSetDropTime(value: $0)) },
                    onRemove: { core.dispatch(event: .matchSetDropTime(value: "")) }
                )
                labelledDrop(
                    "ESPAÇO",
                    value: mv.dropSpace,
                    onDrop: { core.dispatch(event: .matchSetDropSpace(value: $0)) },
                    onRemove: { core.dispatch(event: .matchSetDropSpace(value: "")) }
                )

                Rectangle()
                    .frame(height: 1)
                    .foregroundColor(LognDark.line)
                    .padding(.vertical, 8)

                chipBank(mv.currentOptions.filter { $0 != mv.dropTime && $0 != mv.dropSpace })
            }

        case "TAG_THE_PATTERN":
            VStack(alignment: .leading, spacing: 18) {
                if !mv.currentCodeLines.isEmpty {
                    CodeBlock(lines: mv.currentCodeLines)
                }

                Text("SELECIONE ATÉ \(mv.maxSelections)")
                    .font(.plexMono(11))
                    .tracking(0.12 * 11)
                    .foregroundColor(LognDark.textMuted)

                FlowLayout(spacing: 8) {
                    ForEach(mv.currentOptions, id: \.self) { option in
                        TagChip(
                            text: option,
                            isSelected: mv.selectedTags.contains(option),
                            action: { core.dispatch(event: .matchToggleTag(tag: option)) }
                        )
                    }
                }
            }

        default:
            EmptyView()
        }
    }

    private func labelledDrop(
        _ label: String,
        value: String,
        onDrop: @escaping (String) -> Void,
        onRemove: @escaping () -> Void
    ) -> some View {
        HStack(spacing: 12) {
            Text(label)
                .font(.plexMono(11))
                .tracking(0.12 * 11)
                .foregroundColor(LognDark.textMuted)
                .frame(width: 60, alignment: .leading)

            DropZone(value: value, onDrop: onDrop, onRemove: onRemove)
        }
    }

    private func chipBank(_ options: [String]) -> some View {
        FlowLayout(spacing: 8) {
            ForEach(options, id: \.self) { DraggableChip(text: $0) }
        }
    }

    // MARK: - Submissão

    private var canSubmit: Bool {
        switch mv.currentTemplateType {
        case "SPOT_THE_BUG":      return mv.selectedLine >= 0
        case "FILL_IN_THE_BLANK": return !mv.answerString.isEmpty
        case "TAG_THE_PATTERN":   return mv.selectedTags.count == Int(mv.maxSelections)
        case "COMPLEXITY_MATCH":  return !mv.dropTime.isEmpty && !mv.dropSpace.isEmpty
        case "DRY_RUN":           return !mv.predictedOutput.trimmingCharacters(in: .whitespaces).isEmpty
        default:                  return false
        }
    }

    /// O CTA sempre nomeia a escolha — é ele que torna um mis-tap reversível.
    private var buttonTitle: String {
        switch mv.currentTemplateType {
        case "SPOT_THE_BUG":
            return mv.selectedLine >= 0 ? "Confirmar linha \(mv.selectedLine + 1)" : "Confirmar"
        case "FILL_IN_THE_BLANK":
            return "Confirmar resposta"
        case "TAG_THE_PATTERN":
            return mv.selectedTags.isEmpty
                ? "Confirmar"
                : "Confirmar \(mv.selectedTags.count) tag\(mv.selectedTags.count == 1 ? "" : "s")"
        case "COMPLEXITY_MATCH":
            return "Confirmar complexidade"
        case "DRY_RUN":
            return "Confirmar saída"
        default:
            return "Confirmar"
        }
    }

    private func submit() {
        let letter = currentLetter

        core.dispatch(event: .matchSubmit(timestamp: Int64(Date().timeIntervalSince1970)))

        let code = VerdictChip.Verdict(code: core.viewModel.matchView.lastVerdict)
        let accepted = code == .ac

        UINotificationFeedbackGenerator().notificationOccurred(accepted ? .success : .error)

        withAnimation(.easeOut(duration: 0.12)) {
            verdict = VerdictSnapshot(
                letter: letter,
                code: code,
                livesLeft: Int(core.viewModel.matchView.lives)
            )
        }
    }

    private func dismissVerdict() {
        if mv.hasTrap { core.dispatch(event: .matchDismissTrap) }
        withAnimation(.easeOut(duration: 0.18)) { verdict = nil }
    }

    // MARK: - Relógio

    private func startTimer() {
        timer?.invalidate()
        timer = Timer.scheduledTimer(withTimeInterval: 1.0, repeats: true) { _ in
            // O relógio da questão para durante o veredito e a trap.
            guard mv.isActive, !mv.hasTrap, verdict == nil else { return }
            core.dispatch(event: .matchTimerTick)
        }
    }
}

// MARK: - Veredito em tela cheia

/// `3b · Contest` — o veredito toma a tela inteira. A sigla do juiz é o herói;
/// a punição (vida, penalidade) aparece em números, não em adjetivos.
struct MatchVerdictScreen: View {
    let snapshot: MatchView.VerdictSnapshot
    let reduceMotion: Bool
    let balloonStates: [(Character, Bool)]
    let remainingSeconds: Int
    let lives: Int
    let maxLives: Int
    let trapCategory: String
    let trapTitle: String
    let trapExplanation: String
    let matchOver: Bool
    let xpAward: Int
    let onContinue: () -> Void

    private var letterColor: Color { BalloonColor.forLetter(snapshot.letter) }

    /// Fecho da partida. Quem subiu todos os balões fechou o nó; quem parou no meio
    /// deixou o nó em aberto — e dizer "em aberto" para quem zerou é mentira.
    private var closingNote: String {
        let solved = balloonStates.filter(\.1).count
        return solved == balloonStates.count && !balloonStates.isEmpty
            ? "volta para a trilha · nó completo"
            : "volta para a trilha · nó fica em aberto"
    }

    /// O shake mora aqui, no header do veredito — é onde o mock do `3b` o coloca
    /// (`gp-shake 240ms ease-out`), não na tela de questão.
    @State private var shake: CGFloat = 0
    @State private var errorFlash = false

    var body: some View {
        VStack(spacing: 0) {
            MatchHeader(
                letter: snapshot.letter,
                remainingSeconds: remainingSeconds,
                isFrozen: false,
                lives: lives,
                maxLives: maxLives,
                balloonStates: balloonStates,
                currentIsAlive: snapshot.isAccepted,
                showsBalloonRow: snapshot.isAccepted
            )
            .modifier(ShakeEffect(animatableData: shake))
            // `reduceMotion`: o shake vira flash de borda.
            .overlay(alignment: .bottom) {
                if errorFlash {
                    Rectangle().frame(height: 2).foregroundColor(LognDark.wrong)
                }
            }
            .onAppear {
                guard !snapshot.isAccepted else { return }
                if reduceMotion {
                    errorFlash = true
                    withAnimation(.easeOut(duration: 0.24).delay(0.24)) { errorFlash = false }
                } else {
                    withAnimation(.easeOut(duration: 0.24)) { shake += 1 }
                }
            }

            if snapshot.isAccepted { hitStage } else { missStage }

            explanationPanel
        }
    }

    // MARK: Palco do acerto

    private var hitStage: some View {
        VStack(spacing: 8) {
            BalloonShape(style: .filled(letterColor), width: 52, showString: true)

            Text("AC")
                .font(.plexMonoSemiBold(54))
                .tracking(-0.03 * 54)
                .foregroundColor(LognDark.correctInk)
                .padding(.top, 10)

            Text("BALÃO \(String(snapshot.letter)) NO AR")
                .font(.plexMono(13))
                .tracking(0.12 * 13)
                .foregroundColor(LognDark.correctInk)

            Text("+\(xpAward) XP")
                .font(.plexMonoSemiBold(20))
                .foregroundColor(LognDark.textPrimary)
                .padding(.top, 18)
        }
        .frame(maxWidth: .infinity, maxHeight: .infinity)
        .background(LognDark.tintOk)
        .accessibilityElement(children: .ignore)
        .accessibilityLabel("Accepted. Balão \(snapshot.letter) no ar. Mais \(xpAward) XP.")
    }

    // MARK: Palco do erro

    private var missStage: some View {
        VStack(spacing: 6) {
            Text(snapshot.code.rawValue)
                .font(.plexMonoSemiBold(76))
                .tracking(-0.03 * 76)
                .foregroundColor(LognDark.wrongInk)

            Text(snapshot.code.meaning.uppercased())
                .font(.plexMono(14))
                .tracking(0.12 * 14)
                .foregroundColor(LognDark.wrongInk)

            HStack(spacing: 20) {
                Text("+20 min pen").foregroundColor(LognDark.textPrimary)
                Text("−1 vida").foregroundColor(LognDark.wrongInk)
                Text("0 XP").foregroundColor(LognDark.textMuted)
            }
            .font(.plexMono(12.5))
            .padding(.top, 22)
        }
        .frame(maxWidth: .infinity, maxHeight: .infinity)
        .background(LognDark.tintErr)
        .accessibilityElement(children: .ignore)
        .accessibilityLabel(
            "\(snapshot.code.rawValue), \(snapshot.code.meaning). Mais 20 minutos de penalidade, menos uma vida."
        )
    }

    // MARK: Painel de explicação

    private var explanationPanel: some View {
        VStack(alignment: .leading, spacing: 0) {
            // No acerto o palco já diz tudo (sigla, balão, XP) — o painel é só o CTA.
            // O documento põe a posição no placar aqui; o Core ainda não expõe standings.
            if !snapshot.isAccepted {
                // A causa, nunca o julgamento.
                Text(trapCategory.uppercased())
                    .font(.plexMono(11))
                    .tracking(0.14 * 11)
                    .foregroundColor(LognDark.wrongInk)

                if !trapTitle.isEmpty {
                    Text(trapTitle)
                        .font(.plexSans(15, relativeTo: .callout))
                        .lineSpacing(23 - 15)
                        .foregroundColor(LognDark.textPrimary)
                        .fixedSize(horizontal: false, vertical: true)
                        .padding(.top, 10)
                }

                if !trapExplanation.isEmpty {
                    Text(trapExplanation)
                        .font(.plexSans(14, relativeTo: .subheadline))
                        .lineSpacing(22 - 14)
                        .foregroundColor(LognDark.textSecondary)
                        .fixedSize(horizontal: false, vertical: true)
                        .padding(.top, 10)
                }
            }

            // Acabou a partida, não há próximo problema: o botão prometia um e a
            // linha abaixo dizia o contrário, na mesma tela.
            LognButton(
                title: matchOver ? "Ver o relatório" : (snapshot.isAccepted ? "Próximo problema" : "Continuar"),
                variant: .primary,
                action: onContinue
            )
            .padding(.top, 18)

            if matchOver {
                Text(closingNote)
                    .font(.plexMono(11))
                    .foregroundColor(LognDark.textMuted)
                    .frame(maxWidth: .infinity)
                    .padding(.top, 10)
            }
        }
        .padding(.horizontal, 18)
        .padding(.top, 20)
        .padding(.bottom, 22)
        .frame(maxWidth: .infinity, alignment: .leading)
        .background(LognDark.surfaceRaised)
        .overlay(alignment: .top) {
            Rectangle()
                .frame(height: 1)
                .foregroundColor(snapshot.isAccepted ? LognDark.lineStrong : LognDark.lineStrong)
        }
        .shadow(color: .black.opacity(0.65), radius: 48, y: -16)
    }
}

// MARK: - Relatório pós-partida

/// Tela 4 do DS: `CONTEST ENCERRADO`, o número de aceitos, a fileira A—M e a revisão.
struct MatchReportView: View {
    let onDismiss: () -> Void
    @EnvironmentObject var core: CoreWrapper

    private var mv: LogN.MatchViewModel { core.viewModel.matchView }

    private var balloonStates: [(Character, Bool)] {
        mv.balloonStates.compactMap { s in s.letter.first.map { ($0, s.isAccepted) } }
    }

    var body: some View {
        VStack(spacing: 0) {
            header
            ScrollView { review }
            LognButton(title: "Entendi", variant: .primary, action: onDismiss)
                .padding(.horizontal, Space.screenMargin)
                .padding(.top, Space.md)
                .padding(.bottom, Space.xl)
                .background(LognDark.canvas)
        }
    }

    private var header: some View {
        VStack(alignment: .leading, spacing: 0) {
            Text("CONTEST ENCERRADO")
                .font(.plexMono(11))
                .tracking(0.14 * 11)
                .foregroundColor(LognDark.textMuted)

            HStack(alignment: .firstTextBaseline, spacing: 10) {
                Text("\(mv.solvedCount)")
                    .font(.plexSansSemiBold(40))
                    .tracking(-0.03 * 40)
                    .monospacedDigit()
                    .foregroundColor(LognDark.textPrimary)

                Text("/ \(mv.totalProblems) aceitos · \(mv.penaltyMinutes) pen")
                    .font(.plexMono(15))
                    .monospacedDigit()
                    .foregroundColor(LognDark.textMuted)
            }
            .padding(.top, 8)

            // A fileira em tamanho grande: é o troféu da sessão. Só os problemas que
            // esta partida teve — balão vazio de problema inexistente não é troféu,
            // é promessa falsa.
            HStack(spacing: 7) {
                ForEach(balloonStates.map(\.0), id: \.self) { letter in
                    let accepted = balloonStates.contains { $0.0 == letter && $0.1 }
                    VStack(spacing: 4) {
                        BalloonShape(
                            style: accepted
                                ? .filled(BalloonColor.forLetter(letter))
                                : .outline(LognDark.lineStrong, 4.5),
                            width: 20,
                            showString: true
                        )
                        Text(String(letter))
                            .font(.plexMono(8))
                            .foregroundColor(LognDark.textMuted)
                    }
                    .opacity(accepted ? 1 : 0.55)
                    .accessibilityElement(children: .ignore)
                    .accessibilityLabel("problema \(letter), \(accepted ? "aceito" : "em aberto")")
                }
                Spacer(minLength: 0)
            }
            .padding(.top, 14)
        }
        .frame(maxWidth: .infinity, alignment: .leading)
        .padding(.horizontal, 20)
        .padding(.top, 22)
        .padding(.bottom, 18)
        .background(LognDark.canvas)
        .overlay(alignment: .bottom) {
            Rectangle().frame(height: 1).foregroundColor(LognDark.line)
        }
    }

    private var review: some View {
        let errors = mv.errors

        return VStack(alignment: .leading, spacing: 10) {
            Text(errors.isEmpty ? "REVISÃO" : "REVISÃO · \(errors.count) ERRO\(errors.count == 1 ? "" : "S")")
                .font(.plexMono(11))
                .tracking(0.12 * 11)
                .foregroundColor(LognDark.textMuted)

            if errors.isEmpty {
                // Empty state: uma frase, nunca uma ilustração.
                Text("Nenhum erro nesta sessão.")
                    .font(LognFont.bodyMedium)
                    .foregroundColor(LognDark.textSecondary)
            } else {
                ForEach(Array(errors.enumerated()), id: \.offset) { _, error in
                    ErrorReviewCard(error: error)
                }
            }
        }
        .frame(maxWidth: .infinity, alignment: .leading)
        .padding(.horizontal, 20)
        .padding(.vertical, 18)
    }
}

/// Um card por erro. O veredito é a sigla em mono na tinta do tom — não o chip,
/// que o documento reserva para contextos onde a moldura ajuda a separar.
struct ErrorReviewCard: View {
    let error: LogN.MatchError

    private var verdict: VerdictChip.Verdict { VerdictChip.Verdict(code: error.verdict) }

    var body: some View {
        VStack(alignment: .leading, spacing: 0) {
            HStack(alignment: .top, spacing: 10) {
                Text(error.title)
                    .font(.plexSansSemiBold(15, relativeTo: .callout))
                    .foregroundColor(LognDark.textPrimary)
                    .fixedSize(horizontal: false, vertical: true)

                Spacer(minLength: 0)

                Text(verdict.rawValue)
                    .font(.plexMono(11))
                    .foregroundColor(verdict.ink)
            }

            if !error.givenAnswer.isEmpty {
                Text("sua resposta: \(error.givenAnswer)")
                    .font(.plexMono(12))
                    .foregroundColor(LognDark.textMuted)
                    .padding(.top, 6)
            }

            if !error.explanation.isEmpty {
                Text(error.explanation)
                    .font(.plexSans(13, relativeTo: .footnote))
                    .lineSpacing(13 * 0.5)
                    .foregroundColor(LognDark.textSecondary)
                    .fixedSize(horizontal: false, vertical: true)
                    .padding(.top, 8)
            }
        }
        .padding(14)
        .frame(maxWidth: .infinity, alignment: .leading)
        .background(LognDark.surface)
        .cornerRadius(Radius.sm)
        .overlay(RoundedRectangle(cornerRadius: Radius.sm).stroke(LognDark.line, lineWidth: 1))
        .accessibilityElement(children: .combine)
        .accessibilityLabel(
            "problema \(error.letter), \(error.title), \(verdict.rawValue), \(verdict.meaning)"
        )
    }
}

// MARK: - Chips e dropzones

/// Tag de multi-seleção. Plex Mono 13sp, padding 11×13, raio 2dp.
struct TagChip: View {
    let text: String
    let isSelected: Bool
    let action: () -> Void

    var body: some View {
        Button(action: action) {
            Text(text)
                .font(.plexMono(13))
                .foregroundColor(isSelected ? LognDark.accentInk : LognDark.textSecondary)
                .padding(.horizontal, 13)
                .padding(.vertical, 11)
                // Piso de 56pt em partida: um mis-tap custa uma vida.
                .frame(minHeight: Space.matchTouch)
                .background(isSelected ? LognDark.accentTint : Color.clear)
                .cornerRadius(Radius.xs)
                .overlay(
                    RoundedRectangle(cornerRadius: Radius.xs)
                        .stroke(isSelected ? LognDark.accent : LognDark.lineStrong, lineWidth: 1)
                )
        }
        .buttonStyle(PressSinkStyle())
        .accessibilityAddTraits(isSelected ? [.isSelected] : [])
    }
}

/// Bloco arrastável do banco de opções.
struct DraggableChip: View {
    let text: String

    var body: some View {
        Text(text)
            .font(.plexMono(13))
            .foregroundColor(LognDark.textPrimary)
            .padding(.horizontal, 12)
            .padding(.vertical, 10)
            .frame(minHeight: Space.matchTouch)
            .background(LognDark.surfaceRaised)
            .cornerRadius(Radius.xs)
            .overlay(RoundedRectangle(cornerRadius: Radius.xs).stroke(LognDark.lineStrong, lineWidth: 1))
            .onDrag { NSItemProvider(object: text as NSString) }
    }
}

/// Altura 46dp, raio 2dp. Vazia: tracejado `lineDim`. Arraste por cima: tracejado `accent`
/// com fundo accent. Preenchida: borda sólida `lineStrong`, fundo `surfaceRaised`.
struct DropZone: View {
    let value: String
    let onDrop: (String) -> Void
    let onRemove: () -> Void

    @State private var isTargeted = false

    private var isFilled: Bool { !value.isEmpty }

    var body: some View {
        ZStack {
            RoundedRectangle(cornerRadius: Radius.xs)
                .fill(fillColor)

            RoundedRectangle(cornerRadius: Radius.xs)
                .stroke(strokeColor, style: StrokeStyle(lineWidth: 1, dash: isFilled ? [] : [4]))

            Text(isFilled ? value : "solte aqui")
                .font(.plexMono(isFilled ? 14 : 13))
                .foregroundColor(isFilled ? LognDark.textPrimary : LognDark.textDim)
        }
        .frame(height: 46)
        .frame(maxWidth: .infinity)
        .contentShape(Rectangle())
        .onTapGesture { if isFilled { onRemove() } }
        .onDrop(of: [.plainText], isTargeted: $isTargeted) { providers in
            guard let provider = providers.first else { return false }
            _ = provider.loadObject(ofClass: NSString.self) { object, _ in
                guard let text = object as? NSString else { return }
                DispatchQueue.main.async { onDrop(text as String) }
            }
            return true
        }
        .accessibilityLabel(isFilled ? "preenchido com \(value)" : "solte aqui")
    }

    private var fillColor: Color {
        if isTargeted { return LognDark.accentTint }
        return isFilled ? LognDark.surfaceRaised : LognDark.canvas
    }

    private var strokeColor: Color {
        if isTargeted { return LognDark.accent }
        return isFilled ? LognDark.lineStrong : LognDark.lineDim
    }
}

// MARK: - Layout de chips

/// Quebra de linha para bancos de chips — `LazyVGrid` distribuiria em colunas de largura
/// fixa; aqui cada chip tem a largura do seu texto, como no documento.
struct FlowLayout: Layout {
    var spacing: CGFloat = 8

    func sizeThatFits(proposal: ProposedViewSize, subviews: Subviews, cache: inout ()) -> CGSize {
        let maxWidth = proposal.width ?? .infinity
        var x: CGFloat = 0, y: CGFloat = 0, rowHeight: CGFloat = 0

        for view in subviews {
            let size = view.sizeThatFits(.unspecified)
            if x > 0, x + size.width > maxWidth {
                x = 0
                y += rowHeight + spacing
                rowHeight = 0
            }
            x += size.width + spacing
            rowHeight = max(rowHeight, size.height)
        }
        return CGSize(width: maxWidth == .infinity ? x : maxWidth, height: y + rowHeight)
    }

    func placeSubviews(in bounds: CGRect, proposal: ProposedViewSize, subviews: Subviews, cache: inout ()) {
        var x = bounds.minX, y = bounds.minY, rowHeight: CGFloat = 0

        for view in subviews {
            let size = view.sizeThatFits(.unspecified)
            if x > bounds.minX, x + size.width > bounds.maxX {
                x = bounds.minX
                y += rowHeight + spacing
                rowHeight = 0
            }
            view.place(at: CGPoint(x: x, y: y), proposal: ProposedViewSize(size))
            x += size.width + spacing
            rowHeight = max(rowHeight, size.height)
        }
    }
}
