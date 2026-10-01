package sh.logn.app.ui.tree

import androidx.compose.foundation.Canvas
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxWithConstraints
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.offset
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.Path
import androidx.compose.ui.graphics.PathEffect
import androidx.compose.ui.graphics.StrokeCap
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.layout.layout
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.clearAndSetSemantics
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.role
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.max
import sh.logn.app.ui.components.BalloonShape
import sh.logn.app.ui.components.BalloonStyle
import sh.logn.app.ui.components.Icon
import sh.logn.app.ui.components.LognIcon
import sh.logn.app.ui.theme.LognDark
import sh.logn.app.ui.theme.LognFont
import sh.logn.app.ui.theme.Space
import sh.logn.app.ui.theme.TreeMetrics
import sh.logn.core.LogN.NodeStatus
import sh.logn.core.LogN.SkillNode
import sh.logn.coreshell.i18n.Str
import sh.logn.app.ui.theme.Stroke as LognStroke

/** Onde cada nó fica, em dp, a partir da topologia do Core (`row` / `column`). */
class TreeLayout(
    nodes: List<SkillNode>,
    canvasWidth: Dp,
) {
    data class Placement(
        val centerX: Dp,
        val balloonTop: Dp,
        val balloonWidth: Dp,
        val balloonHeight: Dp,
        val labelHeight: Dp,
    ) {
        val totalHeight: Dp get() = balloonHeight + labelHeight
    }

    private val placements = mutableMapOf<String, Placement>()
    val contentHeight: Dp

    /** Arestas do DAG, já resolvidas em pai → filho. */
    val edges: List<Pair<SkillNode, SkillNode>>

    init {
        // As linhas empilham: cada uma começa depois da etiqueta mais baixa da anterior.
        // Passo fixo não serve, porque o nó ativo é maior e tem etiqueta de duas linhas.
        var rowTop = TreeMetrics.topInset
        var bottom = rowTop
        for ((_, rowNodes) in nodes.groupBy { it.row }.toSortedMap()) {
            val maxBalloon = rowNodes.maxOf { balloonHeight(balloonWidth(it.status)) }
            val rowCenter = rowTop + maxBalloon / 2
            var rowBottom = rowTop
            for (node in rowNodes) {
                val width = balloonWidth(node.status)
                val height = balloonHeight(width)
                val label = if (node.status == NodeStatus.ACTIVE) TreeMetrics.labelActive else TreeMetrics.label
                val top = rowCenter - height / 2
                placements[node.id] =
                    Placement(canvasWidth / 2 + TreeMetrics.columnPitch * node.column, top, width, height, label)
                rowBottom = max(rowBottom, top + height + label)
            }
            bottom = rowBottom
            rowTop = rowBottom + TreeMetrics.stringGap
        }
        contentHeight = bottom + TreeMetrics.bottomInset
        val byId = nodes.associateBy { it.id }
        edges = nodes.flatMap { child -> child.prerequisites.mapNotNull { byId[it]?.let { parent -> parent to child } } }
    }

    fun placement(id: String): Placement? = placements[id]

    companion object {
        /** Larguras de viewBox do DS v2: tamanho por importância. */
        fun balloonWidth(status: NodeStatus): Dp =
            when (status) {
                NodeStatus.COMPLETED -> TreeMetrics.completed
                NodeStatus.ACTIVE -> TreeMetrics.active
                NodeStatus.LOCKED, NodeStatus.PAYWALLLOCKED -> TreeMetrics.locked
            }

        fun balloonHeight(width: Dp): Dp = width * (VIEW_SHORT / VIEW_WIDTH)

        private const val VIEW_SHORT = 95f
        private const val VIEW_WIDTH = 96f
    }
}

/**
 * A trilha como DAG (DS v2, "grafo de balões"). Espelho de SkillTreeView.swift: balões com
 * o ícone do assunto, e as cordinhas amarram cada balão ao pré-requisito.
 */
@Composable
fun SkillTree(
    nodes: List<SkillNode>,
    onNode: (SkillNode) -> Unit,
    modifier: Modifier = Modifier,
) {
    val context = LocalContext.current
    Column(modifier.fillMaxSize()) {
        BoxWithConstraints(
            Modifier
                .weight(1f)
                .fillMaxWidth(),
        ) {
            val layout = remember(nodes, maxWidth) { TreeLayout(nodes, maxWidth) }
            Box(
                Modifier
                    .fillMaxSize()
                    .verticalScroll(rememberScrollState()),
            ) {
                Box(
                    Modifier
                        .fillMaxWidth()
                        .height(layout.contentHeight),
                ) {
                    Edges(layout)
                    for (node in nodes) {
                        val placement = layout.placement(node.id) ?: continue
                        NodeView(
                            node,
                            placement,
                            Modifier
                                .offset(x = placement.centerX - placement.balloonWidth / 2, y = placement.balloonTop)
                                // Bloqueado também abre: a folha diz o que falta.
                                .clickable { onNode(node) },
                        )
                    }
                }
            }
        }
        // Legenda ancorada: explica a gramática do grafo sem tirar espaço do mapa.
        Text(
            Str.Tree.edge_legend(context),
            style = LognFont.mono(TreeMetrics.LEGEND_SIZE, tracking = TreeMetrics.LEGEND_TRACKING),
            color = LognDark.textMuted,
            modifier =
                Modifier
                    .padding(horizontal = TreeMetrics.legendPaddingH)
                    .padding(top = Space.sm, bottom = TreeMetrics.legendBottom),
        )
    }
}

/**
 * As cordinhas, em três estados como na legenda: percorrida (sólida, na cor do pai),
 * ativa (acento) e pré-requisito pendente (tracejada).
 */
@Composable
private fun Edges(layout: TreeLayout) {
    Canvas(Modifier.fillMaxSize()) {
        val dashed = PathEffect.dashPathEffect(floatArrayOf(TreeMetrics.dash.toPx(), TreeMetrics.dashGap.toPx()))
        for ((parent, child) in layout.edges) {
            val from = layout.placement(parent.id) ?: continue
            val to = layout.placement(child.id) ?: continue
            val start = Offset(from.centerX.toPx(), (from.balloonTop + from.totalHeight).toPx())
            val end = Offset(to.centerX.toPx(), to.balloonTop.toPx())
            val slack = (end.y - start.y) * TreeMetrics.EDGE_SLACK
            val path =
                Path().apply {
                    moveTo(start.x, start.y)
                    cubicTo(start.x, start.y + slack, end.x, end.y - slack, end.x, end.y)
                }
            when {
                parent.status != NodeStatus.COMPLETED ->
                    drawPath(path, LognDark.lineDim, style = Stroke(TreeMetrics.edgeWidth.toPx(), cap = StrokeCap.Round, pathEffect = dashed))
                child.status == NodeStatus.ACTIVE ->
                    drawPath(path, LognDark.accent, style = Stroke(TreeMetrics.activeEdgeWidth.toPx(), cap = StrokeCap.Round))
                else ->
                    drawPath(
                        path,
                        LognTopic.of(parent).color.copy(alpha = TreeMetrics.EDGE_ALPHA),
                        style = Stroke(TreeMetrics.edgeWidth.toPx(), cap = StrokeCap.Round),
                    )
            }
        }
    }
}

/** O estilo do balão de um nó, o mesmo na árvore e na folha. */
fun nodeBalloonStyle(
    node: SkillNode,
    topic: LognTopic,
): BalloonStyle =
    when (node.status) {
        NodeStatus.COMPLETED -> BalloonStyle.Filled(topic.color)
        NodeStatus.ACTIVE -> BalloonStyle.Active
        NodeStatus.LOCKED, NodeStatus.PAYWALLLOCKED -> BalloonStyle.Locked
    }

/** A tinta do ícone dentro do balão: escura sobre o corpo cheio. */
fun nodeIconColor(status: NodeStatus): Color =
    when (status) {
        NodeStatus.COMPLETED -> LognDark.onAccent
        NodeStatus.ACTIVE -> LognDark.accentInk
        NodeStatus.LOCKED, NodeStatus.PAYWALLLOCKED -> LognDark.textSecondary
    }

val SkillNode.isClosed: Boolean get() = status == NodeStatus.LOCKED || status == NodeStatus.PAYWALLLOCKED
val SkillNode.solvedCount: Int get() = problemsSolved.count { it }

/** O balão com o ícone do assunto dentro, e a etiqueta pendurada. */
@Composable
private fun NodeView(
    node: SkillNode,
    placement: TreeLayout.Placement,
    modifier: Modifier,
) {
    val context = LocalContext.current
    val topic = LognTopic.of(node)
    val description = nodeAccessibility(context, node)
    Column(
        modifier
            .width(placement.balloonWidth)
            .height(placement.totalHeight)
            .clearAndSetSemantics {
                contentDescription = description
                if (node.status != NodeStatus.LOCKED) role = Role.Button
            },
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        Box(Modifier.size(placement.balloonWidth, placement.balloonHeight)) {
            BalloonShape(nodeBalloonStyle(node, topic), placement.balloonWidth)
            val ratio = if (node.isClosed) TreeMetrics.ICON_RATIO_LOCKED else TreeMetrics.ICON_RATIO
            val iconSize = placement.balloonWidth * ratio
            TopicIcon(
                topic,
                nodeIconColor(node.status),
                iconSize,
                Modifier.offset(
                    x = (placement.balloonWidth - iconSize) / 2,
                    y = placement.balloonWidth * TreeMetrics.ICON_CENTER - iconSize / 2,
                ),
                lineWidth =
                    when (node.status) {
                        NodeStatus.COMPLETED -> TreeMetrics.ICON_STROKE_COMPLETED
                        NodeStatus.ACTIVE -> TreeMetrics.ICON_STROKE_ACTIVE
                        else -> TOPIC_STROKE
                    },
            )
            // Grau de entrada 2 = dois pré-requisitos. O número é a informação.
            if (node.prerequisites.size >= 2) {
                Box(
                    Modifier
                        .align(Alignment.TopEnd)
                        .offset(x = TreeMetrics.degreeOffsetX, y = TreeMetrics.degreeOffsetY)
                        .size(TreeMetrics.degreeBadge)
                        .background(LognDark.canvas, CircleShape)
                        .border(LognStroke.hairline, LognDark.lineDim, CircleShape),
                    contentAlignment = Alignment.Center,
                ) {
                    Text("${node.prerequisites.size}", style = LognFont.mono(TreeMetrics.DEGREE_SIZE), color = LognDark.textSecondary)
                }
            }
        }
        Box(Modifier.height(placement.labelHeight), contentAlignment = Alignment.TopCenter) { NodeLabel(node) }
    }
}

/** A etiqueta: a do nó ativo tem duas linhas (nome e "INFLANDO · n/m"). */
@Composable
private fun NodeLabel(node: SkillNode) {
    val context = LocalContext.current
    val shape = RoundedCornerShape(TreeMetrics.labelRadius)
    val active = node.status == NodeStatus.ACTIVE
    val labelModifier =
        Modifier
            .wrapUnbounded()
            .background(if (active) LognDark.accentTint else LognDark.canvas, shape)
            .border(LognStroke.hairline, if (active) LognDark.accent else LognDark.line, shape)
            .padding(horizontal = TreeMetrics.labelPaddingH, vertical = TreeMetrics.labelPaddingV)
    val nameStyle = LognFont.sans(TreeMetrics.LABEL_SIZE, FontWeight.SemiBold)
    if (active) {
        Column(labelModifier, horizontalAlignment = Alignment.CenterHorizontally) {
            Text(node.name, style = nameStyle, color = LognDark.textPrimary, maxLines = 1, softWrap = false)
            Text(
                if (node.problemsSolved.isEmpty()) {
                    Str.Node.active(context, node.requiredXp)
                } else {
                    Str.Solved.inflating(context, node.solvedCount, node.problemsSolved.size)
                },
                style = LognFont.mono(TreeMetrics.LABEL_SUB_SIZE, tracking = TreeMetrics.LABEL_SUB_TRACKING),
                color = LognDark.accentInk,
                maxLines = 1,
                softWrap = false,
            )
        }
        return
    }
    Row(labelModifier, horizontalArrangement = Arrangement.spacedBy(Space.xs), verticalAlignment = Alignment.CenterVertically) {
        Text(node.name, style = nameStyle, color = if (node.isClosed) LognDark.textSecondary else LognDark.textPrimary, maxLines = 1, softWrap = false)
        // Quantos problemas do nó já renderam XP. No bloqueado, 0/5 é ruído.
        if (!node.isClosed && node.problemsSolved.isNotEmpty()) {
            val all = node.solvedCount == node.problemsSolved.size
            val color = if (all) LognDark.correct else LognDark.textSecondary
            if (all) Icon(LognIcon.Check, color, TreeMetrics.checkIcon, strokeWidth = TreeMetrics.CHECK_STROKE)
            Text("${node.solvedCount}/${node.problemsSolved.size}", style = LognFont.mono(TreeMetrics.COUNT_SIZE), color = color)
        }
    }
}

/** A etiqueta passa da largura do balão: o nome não quebra, como no iOS (`fixedSize`). */
private fun Modifier.wrapUnbounded(): Modifier =
    this.layout { measurable, constraints ->
        val placeable = measurable.measure(constraints.copy(minWidth = 0, maxWidth = UNBOUNDED))
        val width = constraints.maxWidth
        layout(width, placeable.height) { placeable.place((width - placeable.width) / 2, 0) }
    }

/** O balão nunca comunica só por cor: o assunto e o estado vão no rótulo. */
fun nodeAccessibility(
    context: android.content.Context,
    node: SkillNode,
): String =
    when (node.status) {
        NodeStatus.COMPLETED -> Str.Tree.node_completed(context, node.name, node.solvedCount, node.problemsSolved.size)
        NodeStatus.ACTIVE -> Str.Tree.node_active(context, node.name, node.solvedCount, node.problemsSolved.size)
        NodeStatus.PAYWALLLOCKED -> Str.Tree.node_paywalled(context, node.name)
        NodeStatus.LOCKED ->
            if (node.prerequisites.size >= 2) {
                Str.Tree.node_locked_prereq(context, node.name, node.prerequisites.size)
            } else {
                Str.Tree.node_locked(context, node.name)
            }
    }

private const val UNBOUNDED = Int.MAX_VALUE
