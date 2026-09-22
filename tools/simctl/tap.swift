import CoreGraphics
import Foundation

// Toca na tela do simulador usando coordenadas NORMALIZADAS (0..1) da tela do device.
// A área do device na janela é descoberta via Acessibilidade e passada por argumento,
// para o script não depender de onde a janela está.
//
// uso: swift tap.swift <originX> <originY> <w> <h> <nx> <ny> [<nx2> <ny2> ...]

let args = CommandLine.arguments
guard args.count >= 7 else {
    FileHandle.standardError.write("uso: tap.swift originX originY w h nx ny [nx ny ...]\n".data(using: .utf8)!)
    exit(2)
}

let ox = Double(args[1])!, oy = Double(args[2])!
let sw = Double(args[3])!, sh = Double(args[4])!

func tap(_ nx: Double, _ ny: Double) {
    let p = CGPoint(x: ox + nx * sw, y: oy + ny * sh)
    guard let down = CGEvent(mouseEventSource: nil, mouseType: .leftMouseDown,
                             mouseCursorPosition: p, mouseButton: .left),
          let up = CGEvent(mouseEventSource: nil, mouseType: .leftMouseUp,
                           mouseCursorPosition: p, mouseButton: .left),
          let move = CGEvent(mouseEventSource: nil, mouseType: .mouseMoved,
                             mouseCursorPosition: p, mouseButton: .left)
    else { return }

    move.post(tap: .cghidEventTap)
    usleep(120_000)
    down.post(tap: .cghidEventTap)
    usleep(60_000)
    up.post(tap: .cghidEventTap)
    print(String(format: "tap %.3f,%.3f -> %.0f,%.0f", nx, ny, p.x, p.y))
}

/// Arrasto no estilo iOS: pressiona e segura para o `.onDrag` engatar, depois move
/// em passos pequenos (um salto único não gera os eventos intermediários que o
/// SwiftUI precisa) e só então solta.
func drag(_ nx0: Double, _ ny0: Double, _ nx1: Double, _ ny1: Double) {
    let from = CGPoint(x: ox + nx0 * sw, y: oy + ny0 * sh)
    let to = CGPoint(x: ox + nx1 * sw, y: oy + ny1 * sh)

    func post(_ type: CGEventType, _ p: CGPoint) {
        CGEvent(mouseEventSource: nil, mouseType: type, mouseCursorPosition: p, mouseButton: .left)?
            .post(tap: .cghidEventTap)
    }

    post(.mouseMoved, from)
    usleep(150_000)
    post(.leftMouseDown, from)
    usleep(900_000) // long-press: engata o drag

    let steps = 30
    for s in 1...steps {
        let t = Double(s) / Double(steps)
        post(.leftMouseDragged, CGPoint(x: from.x + (to.x - from.x) * t,
                                        y: from.y + (to.y - from.y) * t))
        usleep(25_000)
    }
    usleep(400_000)
    post(.leftMouseUp, to)
    print(String(format: "drag %.0f,%.0f -> %.0f,%.0f", from.x, from.y, to.x, to.y))
}

if args[5] == "drag" {
    drag(Double(args[6])!, Double(args[7])!, Double(args[8])!, Double(args[9])!)
} else {
    var i = 5
    while i + 1 < args.count {
        tap(Double(args[i])!, Double(args[i + 1])!)
        i += 2
        usleep(700_000)
    }
}
