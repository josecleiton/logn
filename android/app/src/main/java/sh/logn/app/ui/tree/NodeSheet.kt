package sh.logn.app.ui.tree

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.IntrinsicSize
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.offset
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.semantics.clearAndSetSemantics
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.text.font.FontWeight
import sh.logn.app.ui.components.BalloonShape
import sh.logn.app.ui.components.BalloonStyle
import sh.logn.app.ui.components.BottomSheet
import sh.logn.app.ui.components.ButtonVariant
import sh.logn.app.ui.components.LognButton
import sh.logn.app.ui.components.balloonColor
import sh.logn.app.ui.theme.LognDark
import sh.logn.app.ui.theme.LognFont
import sh.logn.app.ui.theme.NodeSheetMetrics
import sh.logn.app.ui.theme.Radius
import sh.logn.app.ui.theme.Space
import sh.logn.app.ui.theme.Stroke
import sh.logn.core.LogN.NodeStatus
import sh.logn.core.LogN.SkillNode
import sh.logn.coreshell.i18n.Str

/** O mesmo valor do Core (`XP_PER_ACCEPTED`); só a legenda usa. */
const val XP_PER_ACCEPTED = 50

/**
 * A folha do nó (DS v2, "nó ativo · vizinhança"). Espelho de NodeSheetView.swift: em vez
 * de repetir a descrição, mostra de onde o nó vem e o que ele destrava.
 */
@Composable
fun NodeSheet(
    node: SkillNode,
    incoming: List<SkillNode>,
    unlocks: List<SkillNode>,
    problemCount: Int,
    onDismiss: () -> Unit,
    onStartMatch: () -> Unit,
) {
    val topic = LognTopic.of(node)
    val showsProblems = node.status != NodeStatus.LOCKED && node.problemsSolved.isNotEmpty()
    BottomSheet(onDismiss = onDismiss) {
        Column(
            Modifier
                .fillMaxWidth()
                .padding(start = Space.screenMargin, end = Space.screenMargin, top = Space.lg, bottom = Space.xl),
        ) {
            Box(
                Modifier
                    .align(Alignment.CenterHorizontally)
                    .size(NodeSheetMetrics.handleWidth, NodeSheetMetrics.handleHeight)
                    .background(LognDark.lineStrong, RoundedCornerShape(NodeSheetMetrics.handleHeight)),
            )
            Header(node, topic, Modifier.padding(top = Space.lg))
            Neighbourhood(incoming, unlocks, Modifier.padding(top = Space.screenMargin))
            if (showsProblems) Problems(node, Modifier.padding(top = Space.screenMargin))
            StatStrip(node, unlocks.size, showsProblems, Modifier.padding(top = Space.screenMargin))
            Cta(node, problemCount, Modifier.padding(top = Space.lg), onDismiss, onStartMatch)
        }
    }
}

@Composable
private fun Header(
    node: SkillNode,
    topic: LognTopic,
    modifier: Modifier,
) {
    val context = LocalContext.current
    val (label, color) =
        when (node.status) {
            NodeStatus.COMPLETED -> Str.Node.completed(context) to LognDark.textSecondary
            NodeStatus.ACTIVE -> Str.Node.active(context, node.requiredXp) to LognDark.accentInk
            NodeStatus.LOCKED, NodeStatus.PAYWALLLOCKED -> Str.Node.locked(context) to LognDark.textMuted
        }
    Row(modifier, horizontalArrangement = Arrangement.spacedBy(NodeSheetMetrics.headerGap), verticalAlignment = Alignment.CenterVertically) {
        val width = NodeSheetMetrics.balloon
        Box(Modifier.size(width, TreeLayout.balloonHeight(width))) {
            BalloonShape(nodeBalloonStyle(node, topic), width)
            TopicIcon(
                topic,
                nodeIconColor(node.status),
                NodeSheetMetrics.icon,
                Modifier.offset(
                    x = (width - NodeSheetMetrics.icon) / 2,
                    y = width * sh.logn.app.ui.theme.TreeMetrics.ICON_CENTER - NodeSheetMetrics.icon / 2,
                ),
                lineWidth = sh.logn.app.ui.theme.TreeMetrics.ICON_STROKE_ACTIVE,
            )
        }
        Column(verticalArrangement = Arrangement.spacedBy(Space.xs)) {
            Text(node.name, style = LognFont.sans(NodeSheetMetrics.TITLE_SIZE, FontWeight.SemiBold), color = LognDark.textPrimary)
            // A cor da família vive no balão, não no texto: a paleta é certificada para 3:1
            // como forma, não 4.5:1 como glifo.
            Text(label, style = LognFont.mono(NodeSheetMetrics.STATE_SIZE, tracking = NodeSheetMetrics.STATE_TRACKING), color = color)
        }
    }
}

@Composable
private fun Neighbourhood(
    incoming: List<SkillNode>,
    unlocks: List<SkillNode>,
    modifier: Modifier,
) {
    val context = LocalContext.current
    Row(modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(Space.screenMargin)) {
        NeighbourColumn(Str.Node.upstream(context), incoming, upstream = true, Modifier.weight(1f))
        NeighbourColumn(Str.Node.downstream(context), unlocks, upstream = false, Modifier.weight(1f))
    }
}

@Composable
private fun NeighbourColumn(
    title: String,
    nodes: List<SkillNode>,
    upstream: Boolean,
    modifier: Modifier,
) {
    val context = LocalContext.current
    Column(modifier) {
        Text(title, style = LognFont.mono(NodeSheetMetrics.COLUMN_SIZE, tracking = NodeSheetMetrics.COLUMN_TRACKING), color = LognDark.textMuted)
        if (nodes.isEmpty()) {
            // Vazio nunca é ilustração: uma frase e pronto.
            Text(
                if (upstream) Str.Node.start_point(context) else Str.Node.end_point(context),
                style = LognFont.sans(NodeSheetMetrics.NEIGHBOUR_SIZE),
                color = LognDark.textMuted,
                modifier = Modifier.padding(top = Space.md),
            )
            return@Column
        }
        for ((index, neighbour) in nodes.take(2).withIndex()) {
            val topic = LognTopic.of(neighbour)
            Row(
                Modifier.padding(top = if (index == 0) Space.md else Space.sm),
                horizontalArrangement = Arrangement.spacedBy(NodeSheetMetrics.neighbourGap),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                TopicIcon(topic, if (upstream) topic.color else LognDark.textMuted, NodeSheetMetrics.neighbourIcon)
                Text(
                    neighbour.name,
                    style = LognFont.sans(NodeSheetMetrics.NEIGHBOUR_SIZE),
                    color = if (upstream) LognDark.textPrimary else LognDark.textSecondary,
                    maxLines = 1,
                    overflow = androidx.compose.ui.text.style.TextOverflow.Ellipsis,
                )
            }
        }
    }
}

/**
 * Um balão por problema, na ordem das letras da partida. Cheio é o que já rendeu XP e
 * não paga de novo; em contorno, o que ainda vale.
 */
@Composable
private fun Problems(
    node: SkillNode,
    modifier: Modifier,
) {
    val context = LocalContext.current
    Column(modifier, verticalArrangement = Arrangement.spacedBy(Space.md)) {
        Row(Modifier.fillMaxWidth()) {
            Text(
                Str.Solved.problems(context),
                style = LognFont.mono(NodeSheetMetrics.COLUMN_SIZE, tracking = NodeSheetMetrics.COLUMN_TRACKING),
                color = LognDark.textMuted,
            )
            Spacer(Modifier.weight(1f))
            Text(
                Str.Solved.count(context, node.solvedCount, node.problemsSolved.size),
                style = LognFont.mono(NodeSheetMetrics.STATE_SIZE),
                color = LognDark.textSecondary,
            )
        }
        Row(horizontalArrangement = Arrangement.spacedBy(Space.md)) {
            for ((index, solved) in node.problemsSolved.withIndex()) {
                val letter = 'A' + index
                val label =
                    if (solved) {
                        Str.Solved.problem_done_accessibility(context, letter.toString())
                    } else {
                        Str.Solved.problem_open_accessibility(context, letter.toString(), XP_PER_ACCEPTED)
                    }
                Column(
                    Modifier
                        .width(NodeSheetMetrics.problemSlot)
                        .clearAndSetSemantics { contentDescription = label },
                    horizontalAlignment = Alignment.CenterHorizontally,
                    verticalArrangement = Arrangement.spacedBy(Space.xs),
                ) {
                    val width = NodeSheetMetrics.problemBalloon
                    Box(Modifier.size(width, TreeLayout.balloonHeight(width)), contentAlignment = Alignment.TopCenter) {
                        BalloonShape(
                            if (solved) {
                                BalloonStyle.Filled(balloonColor(letter))
                            } else {
                                BalloonStyle.Outline(balloonColor(letter), NodeSheetMetrics.PROBLEM_OUTLINE)
                            },
                            width,
                            showHighlight = false,
                        )
                        Text(
                            letter.toString(),
                            style = LognFont.mono(NodeSheetMetrics.PROBLEM_LETTER_SIZE, FontWeight.SemiBold),
                            color = if (solved) LognDark.onAccent else LognDark.textSecondary,
                            modifier = Modifier.padding(top = width * PROBLEM_LETTER_TOP),
                        )
                    }
                    Text(
                        if (solved) Str.Solved.done(context) else Str.Solved.worth(context, XP_PER_ACCEPTED),
                        style = LognFont.mono(NodeSheetMetrics.PROBLEM_CAPTION_SIZE, tracking = LABEL_TRACKING),
                        color = if (solved) LognDark.textMuted else LognDark.textPrimary,
                    )
                }
            }
        }
    }
}

@Composable
private fun StatStrip(
    node: SkillNode,
    unlockCount: Int,
    showsProblems: Boolean,
    modifier: Modifier,
) {
    val context = LocalContext.current
    val open = node.problemsSolved.size - node.solvedCount
    val shape = RoundedCornerShape(Radius.sm)
    Row(
        modifier
            .fillMaxWidth()
            .height(IntrinsicSize.Min)
            .background(LognDark.line, shape)
            .border(Stroke.hairline, LognDark.line, shape),
        horizontalArrangement = Arrangement.spacedBy(Stroke.hairline),
    ) {
        StatCell(Str.Node.prereq(context), "${node.prerequisites.size}", Modifier.weight(1f))
        // Destravado, o que importa é quanto o nó ainda paga; bloqueado, o portão.
        if (showsProblems) {
            StatCell(Str.Solved.in_play(context), "${open * XP_PER_ACCEPTED} XP", Modifier.weight(1f))
        } else {
            StatCell(XP_LABEL, "${node.requiredXp}", Modifier.weight(1f))
        }
        StatCell(Str.Node.unlocks(context), "$unlockCount", Modifier.weight(1f))
    }
}

@Composable
fun StatCell(
    label: String,
    value: String,
    modifier: Modifier = Modifier,
    valueSize: Float = NodeSheetMetrics.STAT_VALUE_SIZE,
    valueWeight: FontWeight = FontWeight.Normal,
) {
    Column(
        modifier
            .fillMaxHeight()
            .background(LognDark.surface)
            .padding(horizontal = NodeSheetMetrics.statPaddingH, vertical = NodeSheetMetrics.statPaddingV),
        verticalArrangement = Arrangement.spacedBy(Space.xs),
    ) {
        Text(label, style = LognFont.mono(NodeSheetMetrics.COLUMN_SIZE, tracking = NodeSheetMetrics.STAT_LABEL_TRACKING), color = LognDark.textMuted)
        Text(value, style = LognFont.mono(valueSize, valueWeight), color = LognDark.textPrimary)
    }
}

@Composable
private fun Cta(
    node: SkillNode,
    problemCount: Int,
    modifier: Modifier,
    onDismiss: () -> Unit,
    onStartMatch: () -> Unit,
) {
    val context = LocalContext.current
    when {
        node.status == NodeStatus.LOCKED -> LognButton(Str.Node.locked(context), ButtonVariant.Primary, modifier, enabled = false) {}
        problemCount == 0 ->
            // Nó destravado e vazio existe: o currículo ainda não cobre todos. Dizer que
            // não há é melhor que um botão que promete partida e não faz nada.
            Column(modifier, verticalArrangement = Arrangement.spacedBy(Space.sm), horizontalAlignment = Alignment.CenterHorizontally) {
                LognButton(Str.Node.no_problems(context), ButtonVariant.Primary, enabled = false) {}
                Text(Str.Node.no_problems_desc(context), style = LognFont.mono(NodeSheetMetrics.STATE_SIZE), color = LognDark.textMuted)
            }
        else -> {
            val open = node.problemsSolved.size - node.solvedCount
            // Rejogar só é "de novo" quando não há mais nada a ganhar no nó.
            val title =
                when {
                    node.solvedCount == 0 -> Str.Solved.start(context)
                    open > 0 -> Str.Solved.resume(context)
                    else -> Str.Solved.replay(context)
                }
            LognButton(title, ButtonVariant.Primary, modifier) {
                onDismiss()
                onStartMatch()
            }
        }
    }
}

/** "XP" é a sigla, igual em toda língua, como no iOS. */
private const val XP_LABEL = "XP"
private const val PROBLEM_LETTER_TOP = 0.26f
private const val LABEL_TRACKING = 0.08f
