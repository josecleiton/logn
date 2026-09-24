import SwiftUI
import LogNCoreFFI

/// Assuntos da trilha — a iconografia própria do LogN.
///
/// Definida na exploração `4b · Grafo de balões`, promovida ao design system:
/// quatro famílias, quatro cores emprestadas da paleta de balões. O acento fica
/// **reservado ao nó ativo** — nenhum assunto o usa (exceto o boss, que é o próprio
/// desafio e não um assunto de estudo).
enum LognTopic: CaseIterable {
    case adHoc, arrays, strings
    case twoPointers, binarySearch, slidingWindow, sorting
    case trees, graphs
    case dp, greedy
    case challenge

    /// Família do assunto. Dá a cor e agrupa a legenda.
    enum Family {
        case fundamentos, busca, estruturas, otimizacao, boss

        var color: Color {
            switch self {
            case .fundamentos: return Balloon.of("H")   // âmbar
            case .busca:       return Balloon.of("C")   // azul
            case .estruturas:  return Balloon.of("E")   // violeta
            case .otimizacao:  return Balloon.of("M")   // esmeralda
            case .boss:        return LognDark.accent
            }
        }


    }

    var family: Family {
        switch self {
        case .adHoc, .arrays, .strings:
            return .fundamentos
        case .twoPointers, .binarySearch, .slidingWindow, .sorting:
            return .busca
        case .trees, .graphs:
            return .estruturas
        case .dp, .greedy:
            return .otimizacao
        case .challenge:
            return .boss
        }
    }

    var color: Color { family.color }



    // MARK: - Ícone · 24×24, traço 2, cap e join redondos

    /// Elementos do ícone, colados dos SVGs do design system.
    enum IconElement {
        case stroke(String)                       // path `d`, apenas contorno
        case circle(x: CGFloat, y: CGFloat, r: CGFloat)
    }

    var icon: [IconElement] {
        switch self {
        case .adHoc:
            return [.stroke("M12 3v18M4.5 7.5l15 9M19.5 7.5l-15 9")]
        case .arrays:
            return [.stroke("M2 9h20v6H2zM8 9v6M14 9v6")]
        case .strings:
            return [.stroke("M3 7h18v10H3zM7 11h3M13 11h4M7 14h6")]
        case .twoPointers:
            return [.stroke("M2 12h20M7 8l-4 4 4 4M17 8l4 4-4 4")]
        case .binarySearch:
            return [.stroke("M2 10h20v4H2zM12 3v18M12 6l-3 3M12 6l3 3")]
        case .slidingWindow:
            return [.stroke("M2 10h20v4H2z"), .stroke("M7 6h8v12H7z"), .stroke("M18 8l3 4-3 4")]
        case .sorting:
            return [.stroke("M4 20v-4M9.33 20v-8M14.66 20v-12M20 20v-16")]
        case .trees:
            return [
                .circle(x: 12, y: 5, r: 2.5), .circle(x: 6, y: 19, r: 2.5), .circle(x: 18, y: 19, r: 2.5),
                .stroke("M10.4 7.1L7.6 16.9M13.6 7.1l2.8 9.8"),
            ]
        case .graphs:
            return [
                .circle(x: 5, y: 6, r: 2.5), .circle(x: 19, y: 9, r: 2.5), .circle(x: 9, y: 19, r: 2.5),
                .stroke("M7.4 7.1l9.2 1.4M17.6 11.2l-6.9 6.1M6.4 8.2l2.1 8.4"),
            ]
        case .dp:
            return [
                .stroke("M3 4h18v16H3zM3 10h18M3 15h18M9 4v16M15 4v16"),
                .stroke("M3 4l6 6M9 10l6 5"),
            ]
        case .greedy:
            return [.stroke("M4 20v-5M10 20v-9M16 20v-14M22 20v-7"), .stroke("M13 3l3-2 3 2")]
        case .challenge:
            return [.stroke("M6 21V3M6 4h12l-3 4.5L18 13H6")]
        }
    }

    // MARK: - Resolução a partir do nó

    /// O assunto do nó sai do `topic` que o servidor manda (`adhoc`, `two_pointers`…),
    /// neutro de língua. O nome traduzido não serve de chave: em espanhol três nós
    /// caíam na cor errada.
    static func of(node: SkillNode) -> LognTopic {
        slugs[node.topic] ?? of(nodeName: node.name)
    }

    private static let slugs: [String: LognTopic] = [
        "adhoc": .adHoc, "arrays": .arrays, "strings": .strings,
        "two_pointers": .twoPointers, "binary_search": .binarySearch,
        "sliding_window": .slidingWindow, "sorting": .sorting,
        "trees": .trees, "graphs": .graphs, "dp": .dp, "greedy": .greedy,
        "challenge": .challenge,
    ]

    /// Para nó sem `topic` (conteúdo publicado antes da coluna), o cliente resolve pelo
    /// nome, como fazia antes de o Core expor o campo.
    /// O nome do nó vem do conteúdo, em português, e só as chaves em inglês estavam
    /// aqui: a maioria dos nós caía no `.adHoc` e a árvore inteira virava asterisco.
    static func of(nodeName: String) -> LognTopic {
        let n = nodeName.lowercased()
        if n.contains("two pointer") || n.contains("dois ponteiros") { return .twoPointers }
        if n.contains("binary") || n.contains("binária") { return .binarySearch }
        if n.contains("sliding") || n.contains("janela")  { return .slidingWindow }
        if n.contains("sort") || n.contains("ordena")     { return .sorting }
        // Não-lineares são heaps, union-find e árvores: a família "estruturas".
        if n.contains("tree") || n.contains("árvore") || n.contains("não-linear") || n.contains("nao-linear") {
            return .trees
        }
        if n.contains("graph") || n.contains("grafo")     { return .graphs }
        if n.contains("dp") || n.contains("dinâmic") || n.contains("dynamic") { return .dp }
        if n.contains("greedy") || n.contains("guloso") || n.contains("complete search") { return .greedy }
        if n.contains("desafio") || n.contains("boss") || n.contains("challenge") { return .challenge }
        if n.contains("string")       { return .strings }
        // Pilha, fila e bitmask são indexados por posição, como um array.
        if n.contains("array") || n.contains("linear") || n.contains("bitmask") { return .arrays }
        // Aritmética sobre inteiros fica nos fundamentos.
        if n.contains("número") || n.contains("numero") || n.contains("number") { return .adHoc }
        return .adHoc
    }
}
