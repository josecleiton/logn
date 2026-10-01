package sh.logn.app.ui.tree

import androidx.compose.foundation.Canvas
import androidx.compose.foundation.layout.size
import androidx.compose.runtime.Composable
import androidx.compose.runtime.remember
import androidx.compose.ui.Modifier
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.StrokeCap
import androidx.compose.ui.graphics.StrokeJoin
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.graphics.drawscope.scale
import androidx.compose.ui.graphics.vector.PathParser
import androidx.compose.ui.semantics.clearAndSetSemantics
import androidx.compose.ui.unit.Dp
import sh.logn.app.ui.theme.Balloon
import sh.logn.app.ui.theme.LognDark
import sh.logn.core.LogN.SkillNode

/** Um elemento do ícone de assunto: caminho só de contorno, ou círculo. */
sealed interface TopicPart {
    data class Path(
        val d: String,
    ) : TopicPart

    data class Circle(
        val x: Float,
        val y: Float,
        val r: Float,
    ) : TopicPart
}

/** A família do assunto: dá a cor. O acento fica reservado ao nó ativo e ao boss. */
enum class TopicFamily(
    val color: Color,
) {
    Fundamentos(Balloon.of('H')),
    Busca(Balloon.of('C')),
    Estruturas(Balloon.of('E')),
    Otimizacao(Balloon.of('M')),
    Boss(LognDark.accent),
}

/**
 * Os assuntos da trilha, a iconografia própria do LogN (DS v2, "grafo de balões").
 * Espelho de LognTopics.swift, com os mesmos caminhos 24×24.
 */
enum class LognTopic(
    val family: TopicFamily,
    val parts: List<TopicPart>,
) {
    AdHoc(TopicFamily.Fundamentos, listOf(TopicPart.Path("M12 3v18M4.5 7.5l15 9M19.5 7.5l-15 9"))),
    Arrays(TopicFamily.Fundamentos, listOf(TopicPart.Path("M2 9h20v6H2zM8 9v6M14 9v6"))),
    Strings(TopicFamily.Fundamentos, listOf(TopicPart.Path("M3 7h18v10H3zM7 11h3M13 11h4M7 14h6"))),
    TwoPointers(TopicFamily.Busca, listOf(TopicPart.Path("M2 12h20M7 8l-4 4 4 4M17 8l4 4-4 4"))),
    BinarySearch(TopicFamily.Busca, listOf(TopicPart.Path("M2 10h20v4H2zM12 3v18M12 6l-3 3M12 6l3 3"))),
    SlidingWindow(
        TopicFamily.Busca,
        listOf(TopicPart.Path("M2 10h20v4H2z"), TopicPart.Path("M7 6h8v12H7z"), TopicPart.Path("M18 8l3 4-3 4")),
    ),
    Sorting(TopicFamily.Busca, listOf(TopicPart.Path("M4 20v-4M9.33 20v-8M14.66 20v-12M20 20v-16"))),
    Trees(
        TopicFamily.Estruturas,
        listOf(
            TopicPart.Circle(12f, 5f, 2.5f),
            TopicPart.Circle(6f, 19f, 2.5f),
            TopicPart.Circle(18f, 19f, 2.5f),
            TopicPart.Path("M10.4 7.1L7.6 16.9M13.6 7.1l2.8 9.8"),
        ),
    ),
    Graphs(
        TopicFamily.Estruturas,
        listOf(
            TopicPart.Circle(5f, 6f, 2.5f),
            TopicPart.Circle(19f, 9f, 2.5f),
            TopicPart.Circle(9f, 19f, 2.5f),
            TopicPart.Path("M7.4 7.1l9.2 1.4M17.6 11.2l-6.9 6.1M6.4 8.2l2.1 8.4"),
        ),
    ),
    Dp(
        TopicFamily.Otimizacao,
        listOf(TopicPart.Path("M3 4h18v16H3zM3 10h18M3 15h18M9 4v16M15 4v16"), TopicPart.Path("M3 4l6 6M9 10l6 5")),
    ),
    Greedy(TopicFamily.Otimizacao, listOf(TopicPart.Path("M4 20v-5M10 20v-9M16 20v-14M22 20v-7"), TopicPart.Path("M13 3l3-2 3 2"))),
    Challenge(TopicFamily.Boss, listOf(TopicPart.Path("M6 21V3M6 4h12l-3 4.5L18 13H6"))),
    ;

    val color: Color get() = family.color

    companion object {
        private val slugs =
            mapOf(
                "adhoc" to AdHoc,
                "arrays" to Arrays,
                "strings" to Strings,
                "two_pointers" to TwoPointers,
                "binary_search" to BinarySearch,
                "sliding_window" to SlidingWindow,
                "sorting" to Sorting,
                "trees" to Trees,
                "graphs" to Graphs,
                "dp" to Dp,
                "greedy" to Greedy,
                "challenge" to Challenge,
            )

        /**
         * O assunto sai do `topic` que o servidor manda, neutro de língua: o nome
         * traduzido não serve de chave. Nó sem `topic` (conteúdo anterior à coluna) cai no
         * nome, como no iOS.
         */
        fun of(node: SkillNode): LognTopic = slugs[node.topic] ?: ofName(node.name)

        @Suppress("CyclomaticComplexMethod")
        fun ofName(nodeName: String): LognTopic {
            val n = nodeName.lowercase()

            fun has(vararg words: String) = words.any { it in n }
            return when {
                has("two pointer", "dois ponteiros") -> TwoPointers
                has("binary", "binária") -> BinarySearch
                has("sliding", "janela") -> SlidingWindow
                has("sort", "ordena") -> Sorting
                // Não-lineares são heaps, union-find e árvores: a família "estruturas".
                has("tree", "árvore", "não-linear", "nao-linear") -> Trees
                has("graph", "grafo") -> Graphs
                has("dp", "dinâmic", "dynamic") -> Dp
                has("greedy", "guloso", "complete search") -> Greedy
                has("desafio", "boss", "challenge") -> Challenge
                has("string") -> Strings
                // Pilha, fila e bitmask são indexados por posição, como um array.
                has("array", "linear", "bitmask") -> Arrays
                else -> AdHoc
            }
        }
    }
}

/**
 * O ícone de assunto: viewBox 24, traço em unidades da viewBox (2 no DS; engrossa dentro
 * do balão), pontas e juntas redondas.
 */
@Composable
fun TopicIcon(
    topic: LognTopic,
    color: Color,
    size: Dp,
    modifier: Modifier = Modifier,
    lineWidth: Float = TOPIC_STROKE,
) {
    val paths =
        remember(topic) {
            topic.parts.filterIsInstance<TopicPart.Path>().map { PathParser().parsePathString(it.d).toPath() }
        }
    val circles = remember(topic) { topic.parts.filterIsInstance<TopicPart.Circle>() }
    Canvas(modifier.size(size).clearAndSetSemantics { }) {
        val s = this.size.width / TOPIC_GRID
        val stroke = Stroke(lineWidth, cap = StrokeCap.Round, join = StrokeJoin.Round)
        scale(s, s, pivot = Offset.Zero) {
            for (path in paths) drawPath(path, color, style = stroke)
            for (c in circles) drawCircle(color, c.r, Offset(c.x, c.y), style = stroke)
        }
    }
}

private const val TOPIC_GRID = 24f
const val TOPIC_STROKE = 2f
