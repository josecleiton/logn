import Serde

func serializeArray<T, S: Serializer>(
    value: [T],
    serializer: S,
    serializeElement: (T, S) throws -> Void
) throws {
    try serializer.serialize_len(value: value.count)
    for item in value {
        try serializeElement(item, serializer)
    }
}

func deserializeArray<T, D: Deserializer>(
    deserializer: D,
    deserializeElement: (D) throws -> T
) throws -> [T] {
    let length = try deserializer.deserialize_len()
    var obj: [T] = []
    for _ in 0..<length {
        obj.append(try deserializeElement(deserializer))
    }
    return obj
}

func serializeMap<K, V, S: Serializer>(
    value: [K: V],
    serializer: S,
    serializeEntry: (K, V, S) throws -> Void
) throws {
    try serializer.serialize_len(value: value.count)
    for (key, value) in value {
        try serializeEntry(key, value, serializer)
    }
}

func deserializeMap<K: Hashable, V, D: Deserializer>(
    deserializer: D,
    deserializeEntry: (D) throws -> (K, V)
) throws -> [K: V] {
    let length = try deserializer.deserialize_len()
    var obj: [K: V] = [:]
    for _ in 0..<length {
        let (key, value) = try deserializeEntry(deserializer)
        obj[key] = value
    }
    return obj
}

func serializeOption<T, S: Serializer>(
    value: T?,
    serializer: S,
    serializeElement: (T, S) throws -> Void
) throws {
    if let value = value {
        try serializer.serialize_option_tag(value: true)
        try serializeElement(value, serializer)
    } else {
        try serializer.serialize_option_tag(value: false)
    }
}

func deserializeOption<T, D: Deserializer>(
    deserializer: D,
    deserializeElement: (D) throws -> T
) throws -> T? {
    let tag = try deserializer.deserialize_option_tag()
    if tag {
        return try deserializeElement(deserializer)
    } else {
        return nil
    }
}

public struct BalloonState: Hashable, Equatable {
    public var letter: String
    public var isAccepted: Bool
    /// Já tinha rendido XP antes desta partida.
    public var alreadyPaid: Bool

    public init(letter: String, isAccepted: Bool, alreadyPaid: Bool) {
        self.letter = letter
        self.isAccepted = isAccepted
        self.alreadyPaid = alreadyPaid
    }

    public func serialize<S: Serializer>(serializer: S) throws {
        try serializer.increase_container_depth()
        try serializer.serialize_str(value: self.letter)
        try serializer.serialize_bool(value: self.isAccepted)
        try serializer.serialize_bool(value: self.alreadyPaid)
        try serializer.decrease_container_depth()
    }

    public func bincodeSerialize() throws -> [UInt8] {
        let serializer = BincodeSerializer.init();
        try self.serialize(serializer: serializer)
        return serializer.get_bytes()
    }

    public static func deserialize<D: Deserializer>(deserializer: D) throws -> BalloonState {
        try deserializer.increase_container_depth()
        let letter = try deserializer.deserialize_str()
        let isAccepted = try deserializer.deserialize_bool()
        let alreadyPaid = try deserializer.deserialize_bool()
        try deserializer.decrease_container_depth()
        return BalloonState(letter: letter, isAccepted: isAccepted, alreadyPaid: alreadyPaid)
    }

    public static func bincodeDeserialize(input: [UInt8]) throws -> BalloonState {
        let deserializer = BincodeDeserializer.init(input: input);
        let obj = try deserialize(deserializer: deserializer)
        if deserializer.get_buffer_offset() < input.count {
            throw DeserializationError.invalidInput(issue: "Some input bytes were not read")
        }
        return obj
    }
}

/// Uma das verificações que a abertura roda antes de soltar o jogador no app.
/// 
/// A splash é o log de um juiz: cada verificação imprime o seu veredito numa linha, na
/// ordem, e a splash some quando a última fecha. A ordem é sessão, sync, termos: o sync
/// vem antes para nenhum XP depender do aceite (ADR 0020).
indirect public enum BootCheck: Hashable, Equatable {
    case session
    case sync
    case terms

    public func serialize<S: Serializer>(serializer: S) throws {
        try serializer.increase_container_depth()
        switch self {
        case .session:
            try serializer.serialize_variant_index(value: 0)
        case .sync:
            try serializer.serialize_variant_index(value: 1)
        case .terms:
            try serializer.serialize_variant_index(value: 2)
        }
        try serializer.decrease_container_depth()
    }

    public func bincodeSerialize() throws -> [UInt8] {
        let serializer = BincodeSerializer.init();
        try self.serialize(serializer: serializer)
        return serializer.get_bytes()
    }

    public static func deserialize<D: Deserializer>(deserializer: D) throws -> BootCheck {
        let index = try deserializer.deserialize_variant_index()
        try deserializer.increase_container_depth()
        switch index {
        case 0:
            try deserializer.decrease_container_depth()
            return .session
        case 1:
            try deserializer.decrease_container_depth()
            return .sync
        case 2:
            try deserializer.decrease_container_depth()
            return .terms
        default: throw DeserializationError.invalidInput(issue: "Unknown variant index for BootCheck: \(index)")
        }
    }

    public static func bincodeDeserialize(input: [UInt8]) throws -> BootCheck {
        let deserializer = BincodeDeserializer.init(input: input);
        let obj = try deserialize(deserializer: deserializer)
        if deserializer.get_buffer_offset() < input.count {
            throw DeserializationError.invalidInput(issue: "Some input bytes were not read")
        }
        return obj
    }
}

/// O que a linha tem a dizer, como chave. A frase mora no catálogo.
indirect public enum BootDetail: Hashable, Equatable {
    /// Sessão: falando com o servidor.
    case checking
    /// Sessão: o servidor trocou o token.
    case tokenRenewed
    /// Sessão: sem rede, mas dentro do prazo guardado no aparelho.
    case localTokenValid
    /// Sessão: o servidor recusou, ou o prazo local venceu.
    case sessionEnded
    /// Sync: mandando a fila.
    case sending
    case nothingToSend
    /// Sync: `count` eventos subiram.
    case sent
    /// Sync: sem rede; `count` eventos continuam na fila.
    case noNetwork
    /// Sync: o servidor recusou a fila, ou ela divergiu depois do rebase.
    case rejected
    /// Sync: passou do tempo da abertura. O envio segue por trás.
    case stillSending
    /// Termos: perguntando ao servidor o que a conta tem para aceitar.
    case termsChecking
    /// Termos: a conta aceitou a vigente.
    case termsCurrent
    /// Termos: há versão relevante para aceitar; o app bloqueia.
    case termsChanged
    /// Termos: só mudanças não relevantes; o app avisa uma vez.
    case termsNotice
    /// Termos: sem rede ou sem resposta a tempo; confere na próxima abertura.
    case termsDeferred

    public func serialize<S: Serializer>(serializer: S) throws {
        try serializer.increase_container_depth()
        switch self {
        case .checking:
            try serializer.serialize_variant_index(value: 0)
        case .tokenRenewed:
            try serializer.serialize_variant_index(value: 1)
        case .localTokenValid:
            try serializer.serialize_variant_index(value: 2)
        case .sessionEnded:
            try serializer.serialize_variant_index(value: 3)
        case .sending:
            try serializer.serialize_variant_index(value: 4)
        case .nothingToSend:
            try serializer.serialize_variant_index(value: 5)
        case .sent:
            try serializer.serialize_variant_index(value: 6)
        case .noNetwork:
            try serializer.serialize_variant_index(value: 7)
        case .rejected:
            try serializer.serialize_variant_index(value: 8)
        case .stillSending:
            try serializer.serialize_variant_index(value: 9)
        case .termsChecking:
            try serializer.serialize_variant_index(value: 10)
        case .termsCurrent:
            try serializer.serialize_variant_index(value: 11)
        case .termsChanged:
            try serializer.serialize_variant_index(value: 12)
        case .termsNotice:
            try serializer.serialize_variant_index(value: 13)
        case .termsDeferred:
            try serializer.serialize_variant_index(value: 14)
        }
        try serializer.decrease_container_depth()
    }

    public func bincodeSerialize() throws -> [UInt8] {
        let serializer = BincodeSerializer.init();
        try self.serialize(serializer: serializer)
        return serializer.get_bytes()
    }

    public static func deserialize<D: Deserializer>(deserializer: D) throws -> BootDetail {
        let index = try deserializer.deserialize_variant_index()
        try deserializer.increase_container_depth()
        switch index {
        case 0:
            try deserializer.decrease_container_depth()
            return .checking
        case 1:
            try deserializer.decrease_container_depth()
            return .tokenRenewed
        case 2:
            try deserializer.decrease_container_depth()
            return .localTokenValid
        case 3:
            try deserializer.decrease_container_depth()
            return .sessionEnded
        case 4:
            try deserializer.decrease_container_depth()
            return .sending
        case 5:
            try deserializer.decrease_container_depth()
            return .nothingToSend
        case 6:
            try deserializer.decrease_container_depth()
            return .sent
        case 7:
            try deserializer.decrease_container_depth()
            return .noNetwork
        case 8:
            try deserializer.decrease_container_depth()
            return .rejected
        case 9:
            try deserializer.decrease_container_depth()
            return .stillSending
        case 10:
            try deserializer.decrease_container_depth()
            return .termsChecking
        case 11:
            try deserializer.decrease_container_depth()
            return .termsCurrent
        case 12:
            try deserializer.decrease_container_depth()
            return .termsChanged
        case 13:
            try deserializer.decrease_container_depth()
            return .termsNotice
        case 14:
            try deserializer.decrease_container_depth()
            return .termsDeferred
        default: throw DeserializationError.invalidInput(issue: "Unknown variant index for BootDetail: \(index)")
        }
    }

    public static func bincodeDeserialize(input: [UInt8]) throws -> BootDetail {
        let deserializer = BincodeDeserializer.init(input: input);
        let obj = try deserialize(deserializer: deserializer)
        if deserializer.get_buffer_offset() < input.count {
            throw DeserializationError.invalidInput(issue: "Some input bytes were not read")
        }
        return obj
    }
}

/// Uma linha do log da abertura.
public struct BootLine: Hashable, Equatable {
    public var check: BootCheck
    public var verdict: BootVerdict
    public var detail: BootDetail
    /// Quantos eventos, para `Sent` e `NoNetwork`. Zero no resto.
    public var count: UInt32

    public init(check: BootCheck, verdict: BootVerdict, detail: BootDetail, count: UInt32) {
        self.check = check
        self.verdict = verdict
        self.detail = detail
        self.count = count
    }

    public func serialize<S: Serializer>(serializer: S) throws {
        try serializer.increase_container_depth()
        try self.check.serialize(serializer: serializer)
        try self.verdict.serialize(serializer: serializer)
        try self.detail.serialize(serializer: serializer)
        try serializer.serialize_u32(value: self.count)
        try serializer.decrease_container_depth()
    }

    public func bincodeSerialize() throws -> [UInt8] {
        let serializer = BincodeSerializer.init();
        try self.serialize(serializer: serializer)
        return serializer.get_bytes()
    }

    public static func deserialize<D: Deserializer>(deserializer: D) throws -> BootLine {
        try deserializer.increase_container_depth()
        let check = try LogN.BootCheck.deserialize(deserializer: deserializer)
        let verdict = try LogN.BootVerdict.deserialize(deserializer: deserializer)
        let detail = try LogN.BootDetail.deserialize(deserializer: deserializer)
        let count = try deserializer.deserialize_u32()
        try deserializer.decrease_container_depth()
        return BootLine(check: check, verdict: verdict, detail: detail, count: count)
    }

    public static func bincodeDeserialize(input: [UInt8]) throws -> BootLine {
        let deserializer = BincodeDeserializer.init(input: input);
        let obj = try deserialize(deserializer: deserializer)
        if deserializer.get_buffer_offset() < input.count {
            throw DeserializationError.invalidInput(issue: "Some input bytes were not read")
        }
        return obj
    }
}

/// Como uma linha da abertura está. `Warn` não segura o jogador; `Fail` na sessão
/// manda para o login. `Skipped` é a verificação que não rodou e roda na próxima
/// abertura (termos sem rede).
indirect public enum BootVerdict: Hashable, Equatable {
    case running
    case ok
    case warn
    case fail
    case skipped

    public func serialize<S: Serializer>(serializer: S) throws {
        try serializer.increase_container_depth()
        switch self {
        case .running:
            try serializer.serialize_variant_index(value: 0)
        case .ok:
            try serializer.serialize_variant_index(value: 1)
        case .warn:
            try serializer.serialize_variant_index(value: 2)
        case .fail:
            try serializer.serialize_variant_index(value: 3)
        case .skipped:
            try serializer.serialize_variant_index(value: 4)
        }
        try serializer.decrease_container_depth()
    }

    public func bincodeSerialize() throws -> [UInt8] {
        let serializer = BincodeSerializer.init();
        try self.serialize(serializer: serializer)
        return serializer.get_bytes()
    }

    public static func deserialize<D: Deserializer>(deserializer: D) throws -> BootVerdict {
        let index = try deserializer.deserialize_variant_index()
        try deserializer.increase_container_depth()
        switch index {
        case 0:
            try deserializer.decrease_container_depth()
            return .running
        case 1:
            try deserializer.decrease_container_depth()
            return .ok
        case 2:
            try deserializer.decrease_container_depth()
            return .warn
        case 3:
            try deserializer.decrease_container_depth()
            return .fail
        case 4:
            try deserializer.decrease_container_depth()
            return .skipped
        default: throw DeserializationError.invalidInput(issue: "Unknown variant index for BootVerdict: \(index)")
        }
    }

    public static func bincodeDeserialize(input: [UInt8]) throws -> BootVerdict {
        let deserializer = BincodeDeserializer.init(input: input);
        let obj = try deserialize(deserializer: deserializer)
        if deserializer.get_buffer_offset() < input.count {
            throw DeserializationError.invalidInput(issue: "Some input bytes were not read")
        }
        return obj
    }
}

/// A splash, como a tela precisa dela.
public struct BootViewModel: Hashable, Equatable {
    /// A abertura ainda não terminou: a splash fica na frente de tudo.
    public var inProgress: Bool
    public var lines: [BootLine]
    /// De 0 a 100, para a barra.
    public var progress: UInt8
    /// Sem rede, com a sessão dentro do prazo: a splash para e pergunta se tenta de
    /// novo ou entra com o que está no aparelho.
    public var awaitingOfflineChoice: Bool

    public init(inProgress: Bool, lines: [BootLine], progress: UInt8, awaitingOfflineChoice: Bool) {
        self.inProgress = inProgress
        self.lines = lines
        self.progress = progress
        self.awaitingOfflineChoice = awaitingOfflineChoice
    }

    public func serialize<S: Serializer>(serializer: S) throws {
        try serializer.increase_container_depth()
        try serializer.serialize_bool(value: self.inProgress)
        try serializeArray(value: self.lines, serializer: serializer) { item, serializer in
            try item.serialize(serializer: serializer)
        }
        try serializer.serialize_u8(value: self.progress)
        try serializer.serialize_bool(value: self.awaitingOfflineChoice)
        try serializer.decrease_container_depth()
    }

    public func bincodeSerialize() throws -> [UInt8] {
        let serializer = BincodeSerializer.init();
        try self.serialize(serializer: serializer)
        return serializer.get_bytes()
    }

    public static func deserialize<D: Deserializer>(deserializer: D) throws -> BootViewModel {
        try deserializer.increase_container_depth()
        let inProgress = try deserializer.deserialize_bool()
        let lines = try deserializeArray(deserializer: deserializer) { deserializer in
            try LogN.BootLine.deserialize(deserializer: deserializer)
        }
        let progress = try deserializer.deserialize_u8()
        let awaitingOfflineChoice = try deserializer.deserialize_bool()
        try deserializer.decrease_container_depth()
        return BootViewModel(inProgress: inProgress, lines: lines, progress: progress, awaitingOfflineChoice: awaitingOfflineChoice)
    }

    public static func bincodeDeserialize(input: [UInt8]) throws -> BootViewModel {
        let deserializer = BincodeDeserializer.init(input: input);
        let obj = try deserialize(deserializer: deserializer)
        if deserializer.get_buffer_offset() < input.count {
            throw DeserializationError.invalidInput(issue: "Some input bytes were not read")
        }
        return obj
    }
}

/// O selo do botão Trilhas: a trilha comprada que mais pede atenção.
public struct CatalogBadge: Hashable, Equatable {
    /// `Soon`, `Today` ou `Expired`; qualquer outro é sem selo.
    public var state: OfflineState
    public var daysLeft: UInt32

    public init(state: OfflineState, daysLeft: UInt32) {
        self.state = state
        self.daysLeft = daysLeft
    }

    public func serialize<S: Serializer>(serializer: S) throws {
        try serializer.increase_container_depth()
        try self.state.serialize(serializer: serializer)
        try serializer.serialize_u32(value: self.daysLeft)
        try serializer.decrease_container_depth()
    }

    public func bincodeSerialize() throws -> [UInt8] {
        let serializer = BincodeSerializer.init();
        try self.serialize(serializer: serializer)
        return serializer.get_bytes()
    }

    public static func deserialize<D: Deserializer>(deserializer: D) throws -> CatalogBadge {
        try deserializer.increase_container_depth()
        let state = try LogN.OfflineState.deserialize(deserializer: deserializer)
        let daysLeft = try deserializer.deserialize_u32()
        try deserializer.decrease_container_depth()
        return CatalogBadge(state: state, daysLeft: daysLeft)
    }

    public static func bincodeDeserialize(input: [UInt8]) throws -> CatalogBadge {
        let deserializer = BincodeDeserializer.init(input: input);
        let obj = try deserialize(deserializer: deserializer)
        if deserializer.get_buffer_offset() < input.count {
            throw DeserializationError.invalidInput(issue: "Some input bytes were not read")
        }
        return obj
    }
}

public struct Challenge: Hashable, Equatable {
    public var id: String
    public var nodeId: String
    public var templateType: String
    public var payload: ChallengePayload
    /// De onde o desafio veio, quando não foi escrito para o LogN; vazio é o caso
    /// comum. O `default` mantém compatível o JSON gravado antes de a coluna existir.
    /// O texto por trás deste id vem em `Model::origins`.
    public var origin: String

    public init(id: String, nodeId: String, templateType: String, payload: ChallengePayload, origin: String) {
        self.id = id
        self.nodeId = nodeId
        self.templateType = templateType
        self.payload = payload
        self.origin = origin
    }

    public func serialize<S: Serializer>(serializer: S) throws {
        try serializer.increase_container_depth()
        try serializer.serialize_str(value: self.id)
        try serializer.serialize_str(value: self.nodeId)
        try serializer.serialize_str(value: self.templateType)
        try self.payload.serialize(serializer: serializer)
        try serializer.serialize_str(value: self.origin)
        try serializer.decrease_container_depth()
    }

    public func bincodeSerialize() throws -> [UInt8] {
        let serializer = BincodeSerializer.init();
        try self.serialize(serializer: serializer)
        return serializer.get_bytes()
    }

    public static func deserialize<D: Deserializer>(deserializer: D) throws -> Challenge {
        try deserializer.increase_container_depth()
        let id = try deserializer.deserialize_str()
        let nodeId = try deserializer.deserialize_str()
        let templateType = try deserializer.deserialize_str()
        let payload = try LogN.ChallengePayload.deserialize(deserializer: deserializer)
        let origin = try deserializer.deserialize_str()
        try deserializer.decrease_container_depth()
        return Challenge(id: id, nodeId: nodeId, templateType: templateType, payload: payload, origin: origin)
    }

    public static func bincodeDeserialize(input: [UInt8]) throws -> Challenge {
        let deserializer = BincodeDeserializer.init(input: input);
        let obj = try deserialize(deserializer: deserializer)
        if deserializer.get_buffer_offset() < input.count {
            throw DeserializationError.invalidInput(issue: "Some input bytes were not read")
        }
        return obj
    }
}

public struct ChallengeContent: Hashable, Equatable {
    public var title: String
    /// O enunciado, em texto, na língua em que o servidor mandou. Chegou a ser uma
    /// chave do catálogo (`TrapKey`), e aí nenhum desafio de verdade desserializava:
    /// o conteúdo vem do servidor, não do catálogo da interface.
    public var description: String
    public var codeLines: [String]
    public var options: [String]?
    public var correctOptions: [String]?
    /// Só em DRY_RUN: estado inicial mostrado no painel de watch.
    public var watchVariables: [WatchVariable]?
    /// Só em DRY_RUN: em que ponto o watch foi capturado, ex. "antes da linha 4".
    public var watchNote: String?
    /// Segundos para esta questão, quando ela foge da régua do template.
    /// 
    /// Existe para o desafio atípico — um trace longo demais, um enunciado curto demais
    /// — e não para ser preenchido em todo desafio: o custo é quase todo do template, e
    /// obrigar um número por desafio faz todo mundo copiar o do vizinho.
    public var seconds: Int32?

    public init(title: String, description: String, codeLines: [String], options: [String]?, correctOptions: [String]?, watchVariables: [WatchVariable]?, watchNote: String?, seconds: Int32?) {
        self.title = title
        self.description = description
        self.codeLines = codeLines
        self.options = options
        self.correctOptions = correctOptions
        self.watchVariables = watchVariables
        self.watchNote = watchNote
        self.seconds = seconds
    }

    public func serialize<S: Serializer>(serializer: S) throws {
        try serializer.increase_container_depth()
        try serializer.serialize_str(value: self.title)
        try serializer.serialize_str(value: self.description)
        try serializeArray(value: self.codeLines, serializer: serializer) { item, serializer in
            try serializer.serialize_str(value: item)
        }
        try serializeOption(value: self.options, serializer: serializer) { value, serializer in
            try serializeArray(value: value, serializer: serializer) { item, serializer in
                try serializer.serialize_str(value: item)
            }
        }
        try serializeOption(value: self.correctOptions, serializer: serializer) { value, serializer in
            try serializeArray(value: value, serializer: serializer) { item, serializer in
                try serializer.serialize_str(value: item)
            }
        }
        try serializeOption(value: self.watchVariables, serializer: serializer) { value, serializer in
            try serializeArray(value: value, serializer: serializer) { item, serializer in
                try item.serialize(serializer: serializer)
            }
        }
        try serializeOption(value: self.watchNote, serializer: serializer) { value, serializer in
            try serializer.serialize_str(value: value)
        }
        try serializeOption(value: self.seconds, serializer: serializer) { value, serializer in
            try serializer.serialize_i32(value: value)
        }
        try serializer.decrease_container_depth()
    }

    public func bincodeSerialize() throws -> [UInt8] {
        let serializer = BincodeSerializer.init();
        try self.serialize(serializer: serializer)
        return serializer.get_bytes()
    }

    public static func deserialize<D: Deserializer>(deserializer: D) throws -> ChallengeContent {
        try deserializer.increase_container_depth()
        let title = try deserializer.deserialize_str()
        let description = try deserializer.deserialize_str()
        let codeLines = try deserializeArray(deserializer: deserializer) { deserializer in
            try deserializer.deserialize_str()
        }
        let options = try deserializeOption(deserializer: deserializer) { deserializer in
            try deserializeArray(deserializer: deserializer) { deserializer in
                try deserializer.deserialize_str()
            }
        }
        let correctOptions = try deserializeOption(deserializer: deserializer) { deserializer in
            try deserializeArray(deserializer: deserializer) { deserializer in
                try deserializer.deserialize_str()
            }
        }
        let watchVariables = try deserializeOption(deserializer: deserializer) { deserializer in
            try deserializeArray(deserializer: deserializer) { deserializer in
                try LogN.WatchVariable.deserialize(deserializer: deserializer)
            }
        }
        let watchNote = try deserializeOption(deserializer: deserializer) { deserializer in
            try deserializer.deserialize_str()
        }
        let seconds = try deserializeOption(deserializer: deserializer) { deserializer in
            try deserializer.deserialize_i32()
        }
        try deserializer.decrease_container_depth()
        return ChallengeContent(title: title, description: description, codeLines: codeLines, options: options, correctOptions: correctOptions, watchVariables: watchVariables, watchNote: watchNote, seconds: seconds)
    }

    public static func bincodeDeserialize(input: [UInt8]) throws -> ChallengeContent {
        let deserializer = BincodeDeserializer.init(input: input);
        let obj = try deserialize(deserializer: deserializer)
        if deserializer.get_buffer_offset() < input.count {
            throw DeserializationError.invalidInput(issue: "Some input bytes were not read")
        }
        return obj
    }
}

public struct ChallengePayload: Hashable, Equatable {
    public var content: ChallengeContent
    public var validation: ChallengeValidation

    public init(content: ChallengeContent, validation: ChallengeValidation) {
        self.content = content
        self.validation = validation
    }

    public func serialize<S: Serializer>(serializer: S) throws {
        try serializer.increase_container_depth()
        try self.content.serialize(serializer: serializer)
        try self.validation.serialize(serializer: serializer)
        try serializer.decrease_container_depth()
    }

    public func bincodeSerialize() throws -> [UInt8] {
        let serializer = BincodeSerializer.init();
        try self.serialize(serializer: serializer)
        return serializer.get_bytes()
    }

    public static func deserialize<D: Deserializer>(deserializer: D) throws -> ChallengePayload {
        try deserializer.increase_container_depth()
        let content = try LogN.ChallengeContent.deserialize(deserializer: deserializer)
        let validation = try LogN.ChallengeValidation.deserialize(deserializer: deserializer)
        try deserializer.decrease_container_depth()
        return ChallengePayload(content: content, validation: validation)
    }

    public static func bincodeDeserialize(input: [UInt8]) throws -> ChallengePayload {
        let deserializer = BincodeDeserializer.init(input: input);
        let obj = try deserialize(deserializer: deserializer)
        if deserializer.get_buffer_offset() < input.count {
            throw DeserializationError.invalidInput(issue: "Some input bytes were not read")
        }
        return obj
    }
}

public struct ChallengeValidation: Hashable, Equatable {
    public var validationType: String
    /// Linha do bug **contada a partir de 1**, como a numeração que o jogador vê.
    /// 
    /// É o número que quem escreve o desafio lê na tela; o motor trabalha em índice,
    /// e a conversão acontece uma vez só, ao montar a partida.
    public var correctLine: Int32?
    public var expectedString: String?
    /// O que dizer a quem errou **este** desafio.
    /// 
    /// Sem isto os três erros de uma sessão saíam com o mesmo texto genérico, porque
    /// o texto morava no motor. Quando falta, o motor cai no genérico.
    public var explanation: String?

    public init(validationType: String, correctLine: Int32?, expectedString: String?, explanation: String?) {
        self.validationType = validationType
        self.correctLine = correctLine
        self.expectedString = expectedString
        self.explanation = explanation
    }

    public func serialize<S: Serializer>(serializer: S) throws {
        try serializer.increase_container_depth()
        try serializer.serialize_str(value: self.validationType)
        try serializeOption(value: self.correctLine, serializer: serializer) { value, serializer in
            try serializer.serialize_i32(value: value)
        }
        try serializeOption(value: self.expectedString, serializer: serializer) { value, serializer in
            try serializer.serialize_str(value: value)
        }
        try serializeOption(value: self.explanation, serializer: serializer) { value, serializer in
            try serializer.serialize_str(value: value)
        }
        try serializer.decrease_container_depth()
    }

    public func bincodeSerialize() throws -> [UInt8] {
        let serializer = BincodeSerializer.init();
        try self.serialize(serializer: serializer)
        return serializer.get_bytes()
    }

    public static func deserialize<D: Deserializer>(deserializer: D) throws -> ChallengeValidation {
        try deserializer.increase_container_depth()
        let validationType = try deserializer.deserialize_str()
        let correctLine = try deserializeOption(deserializer: deserializer) { deserializer in
            try deserializer.deserialize_i32()
        }
        let expectedString = try deserializeOption(deserializer: deserializer) { deserializer in
            try deserializer.deserialize_str()
        }
        let explanation = try deserializeOption(deserializer: deserializer) { deserializer in
            try deserializer.deserialize_str()
        }
        try deserializer.decrease_container_depth()
        return ChallengeValidation(validationType: validationType, correctLine: correctLine, expectedString: expectedString, explanation: explanation)
    }

    public static func bincodeDeserialize(input: [UInt8]) throws -> ChallengeValidation {
        let deserializer = BincodeDeserializer.init(input: input);
        let obj = try deserialize(deserializer: deserializer)
        if deserializer.get_buffer_offset() < input.count {
            throw DeserializationError.invalidInput(issue: "Some input bytes were not read")
        }
        return obj
    }
}

/// Represents a duration of time, internally stored as nanoseconds
public struct Duration: Hashable, Equatable {
    public var nanos: UInt64

    public init(nanos: UInt64) {
        self.nanos = nanos
    }

    public func serialize<S: Serializer>(serializer: S) throws {
        try serializer.increase_container_depth()
        try serializer.serialize_u64(value: self.nanos)
        try serializer.decrease_container_depth()
    }

    public func bincodeSerialize() throws -> [UInt8] {
        let serializer = BincodeSerializer.init();
        try self.serialize(serializer: serializer)
        return serializer.get_bytes()
    }

    public static func deserialize<D: Deserializer>(deserializer: D) throws -> Duration {
        try deserializer.increase_container_depth()
        let nanos = try deserializer.deserialize_u64()
        try deserializer.decrease_container_depth()
        return Duration(nanos: nanos)
    }

    public static func bincodeDeserialize(input: [UInt8]) throws -> Duration {
        let deserializer = BincodeDeserializer.init(input: input);
        let obj = try deserialize(deserializer: deserializer)
        if deserializer.get_buffer_offset() < input.count {
            throw DeserializationError.invalidInput(issue: "Some input bytes were not read")
        }
        return obj
    }
}

indirect public enum Effect: Hashable, Equatable {
    case render(RenderOperation)
    case http(HttpRequest)
    case secureStore(KeyValueOperation)
    case telemetry(TelemetryOperation)
    case monitoring(MonitoringOperation)
    case log(LogOperation)
    case time(TimeRequest)
    case storeReview(StoreReviewOperation)

    public func serialize<S: Serializer>(serializer: S) throws {
        try serializer.increase_container_depth()
        switch self {
        case .render(let x):
            try serializer.serialize_variant_index(value: 0)
            try x.serialize(serializer: serializer)
        case .http(let x):
            try serializer.serialize_variant_index(value: 1)
            try x.serialize(serializer: serializer)
        case .secureStore(let x):
            try serializer.serialize_variant_index(value: 2)
            try x.serialize(serializer: serializer)
        case .telemetry(let x):
            try serializer.serialize_variant_index(value: 3)
            try x.serialize(serializer: serializer)
        case .monitoring(let x):
            try serializer.serialize_variant_index(value: 4)
            try x.serialize(serializer: serializer)
        case .log(let x):
            try serializer.serialize_variant_index(value: 5)
            try x.serialize(serializer: serializer)
        case .time(let x):
            try serializer.serialize_variant_index(value: 6)
            try x.serialize(serializer: serializer)
        case .storeReview(let x):
            try serializer.serialize_variant_index(value: 7)
            try x.serialize(serializer: serializer)
        }
        try serializer.decrease_container_depth()
    }

    public func bincodeSerialize() throws -> [UInt8] {
        let serializer = BincodeSerializer.init();
        try self.serialize(serializer: serializer)
        return serializer.get_bytes()
    }

    public static func deserialize<D: Deserializer>(deserializer: D) throws -> Effect {
        let index = try deserializer.deserialize_variant_index()
        try deserializer.increase_container_depth()
        switch index {
        case 0:
            let x = try LogN.RenderOperation.deserialize(deserializer: deserializer)
            try deserializer.decrease_container_depth()
            return .render(x)
        case 1:
            let x = try LogN.HttpRequest.deserialize(deserializer: deserializer)
            try deserializer.decrease_container_depth()
            return .http(x)
        case 2:
            let x = try LogN.KeyValueOperation.deserialize(deserializer: deserializer)
            try deserializer.decrease_container_depth()
            return .secureStore(x)
        case 3:
            let x = try LogN.TelemetryOperation.deserialize(deserializer: deserializer)
            try deserializer.decrease_container_depth()
            return .telemetry(x)
        case 4:
            let x = try LogN.MonitoringOperation.deserialize(deserializer: deserializer)
            try deserializer.decrease_container_depth()
            return .monitoring(x)
        case 5:
            let x = try LogN.LogOperation.deserialize(deserializer: deserializer)
            try deserializer.decrease_container_depth()
            return .log(x)
        case 6:
            let x = try LogN.TimeRequest.deserialize(deserializer: deserializer)
            try deserializer.decrease_container_depth()
            return .time(x)
        case 7:
            let x = try LogN.StoreReviewOperation.deserialize(deserializer: deserializer)
            try deserializer.decrease_container_depth()
            return .storeReview(x)
        default: throw DeserializationError.invalidInput(issue: "Unknown variant index for Effect: \(index)")
        }
    }

    public static func bincodeDeserialize(input: [UInt8]) throws -> Effect {
        let deserializer = BincodeDeserializer.init(input: input);
        let obj = try deserialize(deserializer: deserializer)
        if deserializer.get_buffer_offset() < input.count {
            throw DeserializationError.invalidInput(issue: "Some input bytes were not read")
        }
        return obj
    }
}

indirect public enum Event: Hashable, Equatable {
    case setLocale(String)
    case telemetrySent
    case ping
    case pong
    /// A senha viaja como a pessoa digitou, sobre TLS; quem roda o Argon2 é o servidor.
    case login(email: String, password: String)
    case loginCompleted(HttpResult)
    /// O shell fez o login no provedor e entrega o ID token e o nonce cru que gerou
    /// (ADR 0016). O pedido ao provedor levou o SHA-256 do nonce; o servidor confere.
    case socialLogin(provider: String, idToken: String, nonce: String)
    case socialLoginCompleted(HttpResult)
    /// Na tela de idade e termos do primeiro login pelo provedor. Mesmo sentido de
    /// `legal_accepted` do `Register`.
    case completeSocialSignup(ageConfirmed: Bool, legalAccepted: Bool)
    /// Fechou a tela de idade e termos sem aceitar: o login pelo provedor acaba.
    case cancelSocialSignup
    /// O login no provedor falhou no aparelho, antes de chegar ao servidor (rede, troca
    /// do código). Cancelar pelo próprio jogador não manda isto.
    case socialLoginFailed
    /// O shell fez o login no GitHub e entrega o código, o verifier do PKCE e o nonce
    /// cru que gerou (ADR 0019). O GitHub não emite ID token: o Core troca o código pelo
    /// bilhete no servidor e segue como `SocialLogin`.
    case gitHubCodeReceived(code: String, codeVerifier: String, nonce: String)
    case gitHubExchanged(HttpResult)
    case continueAsGuest
    case tokenStored(KeyValueResult)
    case accountEmailStored(KeyValueResult)
    case accountEmailRead(KeyValueResult)
    case attemptRefresh
    case tokenRead(KeyValueResult)
    case tokenCleared(KeyValueResult)
    case refreshCompleted(HttpResult)
    case logout
    case fetchChallenges
    case fetchNodes
    case fetchProgress
    case progressFetched(HttpResult)
    case nodesFetched(HttpResult)
    case challengesFetched(HttpResult)
    /// O shell dá o `identifierForVendor` na abertura. Vai no `X-Device-ID` da licença.
    case setDeviceId(String)
    /// O catálogo de trilhas pagas.
    case fetchTracks
    case tracksFetched(HttpResult)
    /// A loja entregou uma transação verificada. O Core manda ao servidor, e só depois
    /// de ele confirmar o shell pode finalizar a transação na loja (spec, seção 5).
    /// `restore` é o "Restaurar compras": a transação já foi finalizada antes.
    /// `product_id` diz de que trilha é a compra, para a tela do passo a passo (F3).
    /// 
    /// `provider` é a loja (ADR 0022): vazio é a App Store, que prova a compra pelo `jws`;
    /// `google_play` prova pelo `purchase_token`, e o `transaction_id` é o próprio token.
    case submitPurchase(jws: String, transactionId: String, productId: String, restore: Bool, provider: String, purchaseToken: String)
    case purchaseSubmitted(jws: String, transactionId: String, productId: String, restore: Bool, provider: String, purchaseToken: String, result: HttpResult)
    /// O shell finalizou a transação na loja.
    case purchaseFinished(transactionId: String)
    /// Fechou a tela do passo a passo da compra. Antes de pronta, a compra segue em
    /// segundo plano.
    case closePurchaseFlow
    /// O jogador tocou em comprar este produto: a compra dele abre o passo a passo.
    case purchaseIntent(productId: String)
    /// "Restaurar compras" começou com `count` transações do Apple ID.
    case restoreStarted(count: UInt32)
    case dismissRestoreResult
    /// A árvore passa a mostrar esta trilha.
    case selectTrack(trackId: String)
    /// "Agora não" na oferta do fim da amostra: ela não volta nesta sessão.
    case dismissSampleOffer(trackId: String)
    /// Abriu o catálogo: o selo de "N dias" do botão Trilhas some até amanhã.
    case catalogOpened
    /// Fuso do aparelho, em segundos. O "hoje" do selo é o do jogador, não o UTC.
    case setUtcOffset(seconds: Int32)
    /// Onboarding fechado ("Por onde começar?"). Não volta mais neste aparelho.
    case completeOnboarding
    /// Abertura: lê a trilha escolhida, o onboarding e o dia do selo.
    case restorePreferences
    case preferenceRestored(key: String, result: KeyValueResult)
    /// Pede a licença da trilha: na compra, ao baixar e a cada abertura com rede.
    case fetchLicense(trackId: String)
    /// `user_id` é de quem pediu: a resposta que chega depois de trocar de conta não
    /// entra na conta nova.
    case licenseFetched(userId: String, trackId: String, result: HttpResult)
    case fetchPackage(trackId: String)
    case packageFetched(userId: String, trackId: String, result: HttpResult)
    /// A maior hora que este aparelho já viu, guardada com as licenças.
    case clockRead(userId: String, result: KeyValueResult)
    /// Lê do aparelho as trilhas que a conta baixou, e revalida com o servidor.
    case loadTrackDownloads
    case trackIndexRead(userId: String, result: KeyValueResult)
    case licenseRead(userId: String, trackId: String, result: KeyValueResult)
    case packageRead(userId: String, trackId: String, result: KeyValueResult)
    /// Apaga do aparelho a chave e o pacote da trilha. A compra continua na conta.
    case deleteTrackDownload(trackId: String)
    case syncNow
    case syncAndLogout
    /// A fila guardada sob a chave de `owner`. Chega depois de `claim_queue` e é
    /// descartada se o dono mudou no meio do caminho.
    case offlineQueueRestored(owner: String, result: KeyValueResult)
    /// A trilha que viaja no bundle do app, entregue pelo shell na abertura.
    /// 
    /// O Core não lê arquivo; quem lê é o shell, e aqui só se decide se a semente serve.
    case bundledTrailLoaded(json: String)
    /// Tocou no X. Pede confirmação quando há partida a perder; sai direto quando não há.
    case leaveMatch
    /// Confirmou a saída no cartão.
    case confirmLeaveMatch
    /// Desistiu de sair e voltou para a partida.
    case cancelLeaveMatch
    /// Tocou no selo de origem. Na primeira vez de cada origem, para o relógio.
    case openOriginSheet
    case closeOriginSheet
    case originsSeenRestored(KeyValueResult)
    case originsSeenStored(KeyValueResult)
    case dismissPasswordReset
    /// O shell dá a hora ao Core: na abertura e ao voltar para o primeiro plano.
    case tick(now: Int64)
    /// Hora pedida pelo `crux_time` enquanto há bloqueio por 429, em segundos desde a
    /// época. Ancora o bloqueio que acabou de começar e recalcula a contagem.
    case cooldownClock(now: Int64)
    /// Passou um segundo de bloqueio: hora de pedir a hora de novo.
    case cooldownElapsed
    case sessionExpiryStored(KeyValueResult)
    case offlineSessionChecked(KeyValueResult)
    case snapshotSaved(KeyValueResult)
    case snapshotRestored(KeyValueResult)
    case rotatedTokenStored(KeyValueResult)
    case attemptRefreshDone
    case syncCompleted(HttpResult)
    case requestOtp(email: String, purpose: String)
    case otpRequested(HttpResult)
    case verifyOtp(email: String, code: String, purpose: String)
    case otpVerified(HttpResult)
    /// `legal_accepted` é a caixa dos termos e da política. Quais versões e em que
    /// língua quem decide é o Core, com o que `FetchLegalVersions` trouxe: o shell não
    /// tem como saber a versão vigente.
    case register(email: String, password: String, otp: String, ageConfirmed: Bool, legalAccepted: Bool)
    case registerCompleted(HttpResult)
    case resetPassword(email: String, newPassword: String, otp: String)
    case resetPasswordCompleted(HttpResult)
    case queueSavedForSync(KeyValueResult)
    case startMatch(nodeId: String)
    case matchSelectLine(line: Int32)
    case matchSetAnswer(answer: String)
    case matchSetDropTime(value: String)
    case matchSetDropSpace(value: String)
    case matchToggleTag(tag: String)
    case matchSetOutput(value: String)
    /// TRADEOFF_MATCH: a opção tocada vai para a primeira casa vazia.
    case matchPickTradeoff(value: String)
    case matchClearBenefit
    case matchClearDrawback
    case matchSubmit(timestamp: Int64)
    case matchDismissTrap
    case matchTimerTick
    case undoLogout
    case dismissLogoutNotice
    /// O refresh token voltou ao cofre depois de um `UndoLogout`.
    case logoutUndone(KeyValueResult)
    /// Hora pedida ao `crux_time` para fechar a partida abandonada. `model.now` pode
    /// ter horas — é a da abertura —, e o fim da partida vai para o histórico.
    case matchAbandonedAt(solved: Int32, now: Int64)
    /// O jogador fechou o relatório pós-partida e volta para a trilha.
    case matchReportClosed
    case reviewMilestonesFired(KeyValueResult)
    case reviewMilestonesRestored(KeyValueResult)
    /// A tela de cadastro abriu: busca as versões vigentes dos termos e da política,
    /// que são as que o cadastro vai aceitar, e a idade mínima do país (ISO 3166-1
    /// alfa-2 ou alfa-3, vazio se o shell não souber). O país vai também no cadastro.
    case fetchLegalVersions(country: String)
    case legalVersionsFetched(HttpResult)
    /// Pede a exclusão da conta, com a senha. Como no `Login`, viaja a senha em si,
    /// sobre TLS, e o servidor confere com Argon2.
    case deleteAccount(password: String)
    /// Pede a exclusão provando que é dono com um login novo no provedor: é o caminho
    /// da conta que não tem senha. `authorization_code` é o da Apple, com que o servidor
    /// revoga o acesso; vazio no Google.
    case deleteAccountWithProvider(provider: String, idToken: String, nonce: String, authorizationCode: String)
    /// A exclusão confirmada por um login novo no GitHub (ADR 0019). A troca devolve o
    /// bilhete e o access token, que vai como `authorization_code` para o servidor
    /// revogar a autorização.
    case deleteAccountWithGitHub(code: String, codeVerifier: String, nonce: String)
    case gitHubDeleteExchanged(HttpResult)
    case accountDeleted(HttpResult)
    /// Fechou a tela "conta desativada, apagada até DD/MM".
    case dismissDeletionNotice
    /// Fechou o aviso de que a exclusão foi cancelada pelo login.
    case dismissAccountRestoredNotice
    /// O interruptor "Análise de uso" do perfil.
    case setAnalyticsEnabled(Bool)
    /// Abertura do app: lê a escolha do interruptor guardada no aparelho.
    case restoreAnalyticsPreference
    case analyticsPreferenceRestored(KeyValueResult)
    /// Abertura do app: confere a sessão, traz a fila de quem é a sessão e manda o que
    /// estiver nela. É o que a splash mostra, linha a linha.
    case startBoot
    /// Passou o tempo que a abertura espera pela rede numa das verificações.
    /// `attempt` é a tentativa que pôs o relógio para correr: o de uma tentativa
    /// anterior não corta a de agora.
    case bootWatchdogElapsed(check: BootCheck, attempt: UInt32)
    /// Passou o tempo que a entrada pelo login espera pela trilha. `attempt` como no da
    /// abertura.
    case enterWatchdogElapsed(attempt: UInt32)
    /// A versão do app e a plataforma, que o aceite dos termos grava. O shell manda na
    /// abertura, junto da língua.
    case setClientInfo(appVersion: String, platform: String)
    /// Pergunta ao servidor o que a conta tem para aceitar (ADR 0020). A abertura roda
    /// depois do sync; o login, quando a sessão fica de pé.
    case fetchTermsPending
    /// `owner` é a conta que perguntou: a resposta de uma conta que já saiu não decide o
    /// bloqueio da que entrou depois.
    case termsPendingFetched(owner: String, result: HttpResult)
    /// "Aceitar e continuar" na tela de novo aceite. A caixa é da tela: o botão só
    /// manda isto marcada.
    case acceptTerms
    case termsAccepted(HttpResult)
    /// O aceite da faixa de mudanças não relevantes, mandado quando ela aparece.
    case termsNoticeAccepted(HttpResult)
    /// Fechou a faixa de mudanças não relevantes.
    case dismissTermsNotice
    /// Sem rede na abertura: tentar de novo.
    case retryBoot
    /// Sem rede na abertura: entrar com o que está no aparelho.
    case continueOffline
    /// O e-mail da sessão que acabou, para o login já vir preenchido.
    case resumeEmailRead(KeyValueResult)
    /// `account_user_id` do aparelho: de quem é a sessão que abriu sem rede.
    case queueOwnerRead(KeyValueResult)
    /// Uma fila de outro dono (visitante, ou a chave de antes das filas por conta),
    /// lida para ser adotada pelo dono atual.
    case queueToAdoptRead(from: String, result: KeyValueResult)
    /// A fila adotada já está gravada sob o dono novo: a chave antiga pode sair.
    case queueAdopted(from: String)
    /// A fila de `owner` lida do disco, para tirar dela os eventos que um sync de antes
    /// da troca de dono já entregou.
    case syncedQueueRead(owner: String, sent: [String], result: KeyValueResult)

    public func serialize<S: Serializer>(serializer: S) throws {
        try serializer.increase_container_depth()
        switch self {
        case .setLocale(let x):
            try serializer.serialize_variant_index(value: 0)
            try serializer.serialize_str(value: x)
        case .telemetrySent:
            try serializer.serialize_variant_index(value: 1)
        case .ping:
            try serializer.serialize_variant_index(value: 2)
        case .pong:
            try serializer.serialize_variant_index(value: 3)
        case .login(let email, let password):
            try serializer.serialize_variant_index(value: 4)
            try serializer.serialize_str(value: email)
            try serializer.serialize_str(value: password)
        case .loginCompleted(let x):
            try serializer.serialize_variant_index(value: 5)
            try x.serialize(serializer: serializer)
        case .socialLogin(let provider, let idToken, let nonce):
            try serializer.serialize_variant_index(value: 6)
            try serializer.serialize_str(value: provider)
            try serializer.serialize_str(value: idToken)
            try serializer.serialize_str(value: nonce)
        case .socialLoginCompleted(let x):
            try serializer.serialize_variant_index(value: 7)
            try x.serialize(serializer: serializer)
        case .completeSocialSignup(let ageConfirmed, let legalAccepted):
            try serializer.serialize_variant_index(value: 8)
            try serializer.serialize_bool(value: ageConfirmed)
            try serializer.serialize_bool(value: legalAccepted)
        case .cancelSocialSignup:
            try serializer.serialize_variant_index(value: 9)
        case .socialLoginFailed:
            try serializer.serialize_variant_index(value: 10)
        case .gitHubCodeReceived(let code, let codeVerifier, let nonce):
            try serializer.serialize_variant_index(value: 11)
            try serializer.serialize_str(value: code)
            try serializer.serialize_str(value: codeVerifier)
            try serializer.serialize_str(value: nonce)
        case .gitHubExchanged(let x):
            try serializer.serialize_variant_index(value: 12)
            try x.serialize(serializer: serializer)
        case .continueAsGuest:
            try serializer.serialize_variant_index(value: 13)
        case .tokenStored(let x):
            try serializer.serialize_variant_index(value: 14)
            try x.serialize(serializer: serializer)
        case .accountEmailStored(let x):
            try serializer.serialize_variant_index(value: 15)
            try x.serialize(serializer: serializer)
        case .accountEmailRead(let x):
            try serializer.serialize_variant_index(value: 16)
            try x.serialize(serializer: serializer)
        case .attemptRefresh:
            try serializer.serialize_variant_index(value: 17)
        case .tokenRead(let x):
            try serializer.serialize_variant_index(value: 18)
            try x.serialize(serializer: serializer)
        case .tokenCleared(let x):
            try serializer.serialize_variant_index(value: 19)
            try x.serialize(serializer: serializer)
        case .refreshCompleted(let x):
            try serializer.serialize_variant_index(value: 20)
            try x.serialize(serializer: serializer)
        case .logout:
            try serializer.serialize_variant_index(value: 21)
        case .fetchChallenges:
            try serializer.serialize_variant_index(value: 22)
        case .fetchNodes:
            try serializer.serialize_variant_index(value: 23)
        case .fetchProgress:
            try serializer.serialize_variant_index(value: 24)
        case .progressFetched(let x):
            try serializer.serialize_variant_index(value: 25)
            try x.serialize(serializer: serializer)
        case .nodesFetched(let x):
            try serializer.serialize_variant_index(value: 26)
            try x.serialize(serializer: serializer)
        case .challengesFetched(let x):
            try serializer.serialize_variant_index(value: 27)
            try x.serialize(serializer: serializer)
        case .setDeviceId(let x):
            try serializer.serialize_variant_index(value: 28)
            try serializer.serialize_str(value: x)
        case .fetchTracks:
            try serializer.serialize_variant_index(value: 29)
        case .tracksFetched(let x):
            try serializer.serialize_variant_index(value: 30)
            try x.serialize(serializer: serializer)
        case .submitPurchase(let jws, let transactionId, let productId, let restore, let provider, let purchaseToken):
            try serializer.serialize_variant_index(value: 31)
            try serializer.serialize_str(value: jws)
            try serializer.serialize_str(value: transactionId)
            try serializer.serialize_str(value: productId)
            try serializer.serialize_bool(value: restore)
            try serializer.serialize_str(value: provider)
            try serializer.serialize_str(value: purchaseToken)
        case .purchaseSubmitted(let jws, let transactionId, let productId, let restore, let provider, let purchaseToken, let result):
            try serializer.serialize_variant_index(value: 32)
            try serializer.serialize_str(value: jws)
            try serializer.serialize_str(value: transactionId)
            try serializer.serialize_str(value: productId)
            try serializer.serialize_bool(value: restore)
            try serializer.serialize_str(value: provider)
            try serializer.serialize_str(value: purchaseToken)
            try result.serialize(serializer: serializer)
        case .purchaseFinished(let transactionId):
            try serializer.serialize_variant_index(value: 33)
            try serializer.serialize_str(value: transactionId)
        case .closePurchaseFlow:
            try serializer.serialize_variant_index(value: 34)
        case .purchaseIntent(let productId):
            try serializer.serialize_variant_index(value: 35)
            try serializer.serialize_str(value: productId)
        case .restoreStarted(let count):
            try serializer.serialize_variant_index(value: 36)
            try serializer.serialize_u32(value: count)
        case .dismissRestoreResult:
            try serializer.serialize_variant_index(value: 37)
        case .selectTrack(let trackId):
            try serializer.serialize_variant_index(value: 38)
            try serializer.serialize_str(value: trackId)
        case .dismissSampleOffer(let trackId):
            try serializer.serialize_variant_index(value: 39)
            try serializer.serialize_str(value: trackId)
        case .catalogOpened:
            try serializer.serialize_variant_index(value: 40)
        case .setUtcOffset(let seconds):
            try serializer.serialize_variant_index(value: 41)
            try serializer.serialize_i32(value: seconds)
        case .completeOnboarding:
            try serializer.serialize_variant_index(value: 42)
        case .restorePreferences:
            try serializer.serialize_variant_index(value: 43)
        case .preferenceRestored(let key, let result):
            try serializer.serialize_variant_index(value: 44)
            try serializer.serialize_str(value: key)
            try result.serialize(serializer: serializer)
        case .fetchLicense(let trackId):
            try serializer.serialize_variant_index(value: 45)
            try serializer.serialize_str(value: trackId)
        case .licenseFetched(let userId, let trackId, let result):
            try serializer.serialize_variant_index(value: 46)
            try serializer.serialize_str(value: userId)
            try serializer.serialize_str(value: trackId)
            try result.serialize(serializer: serializer)
        case .fetchPackage(let trackId):
            try serializer.serialize_variant_index(value: 47)
            try serializer.serialize_str(value: trackId)
        case .packageFetched(let userId, let trackId, let result):
            try serializer.serialize_variant_index(value: 48)
            try serializer.serialize_str(value: userId)
            try serializer.serialize_str(value: trackId)
            try result.serialize(serializer: serializer)
        case .clockRead(let userId, let result):
            try serializer.serialize_variant_index(value: 49)
            try serializer.serialize_str(value: userId)
            try result.serialize(serializer: serializer)
        case .loadTrackDownloads:
            try serializer.serialize_variant_index(value: 50)
        case .trackIndexRead(let userId, let result):
            try serializer.serialize_variant_index(value: 51)
            try serializer.serialize_str(value: userId)
            try result.serialize(serializer: serializer)
        case .licenseRead(let userId, let trackId, let result):
            try serializer.serialize_variant_index(value: 52)
            try serializer.serialize_str(value: userId)
            try serializer.serialize_str(value: trackId)
            try result.serialize(serializer: serializer)
        case .packageRead(let userId, let trackId, let result):
            try serializer.serialize_variant_index(value: 53)
            try serializer.serialize_str(value: userId)
            try serializer.serialize_str(value: trackId)
            try result.serialize(serializer: serializer)
        case .deleteTrackDownload(let trackId):
            try serializer.serialize_variant_index(value: 54)
            try serializer.serialize_str(value: trackId)
        case .syncNow:
            try serializer.serialize_variant_index(value: 55)
        case .syncAndLogout:
            try serializer.serialize_variant_index(value: 56)
        case .offlineQueueRestored(let owner, let result):
            try serializer.serialize_variant_index(value: 57)
            try serializer.serialize_str(value: owner)
            try result.serialize(serializer: serializer)
        case .bundledTrailLoaded(let json):
            try serializer.serialize_variant_index(value: 58)
            try serializer.serialize_str(value: json)
        case .leaveMatch:
            try serializer.serialize_variant_index(value: 59)
        case .confirmLeaveMatch:
            try serializer.serialize_variant_index(value: 60)
        case .cancelLeaveMatch:
            try serializer.serialize_variant_index(value: 61)
        case .openOriginSheet:
            try serializer.serialize_variant_index(value: 62)
        case .closeOriginSheet:
            try serializer.serialize_variant_index(value: 63)
        case .originsSeenRestored(let x):
            try serializer.serialize_variant_index(value: 64)
            try x.serialize(serializer: serializer)
        case .originsSeenStored(let x):
            try serializer.serialize_variant_index(value: 65)
            try x.serialize(serializer: serializer)
        case .dismissPasswordReset:
            try serializer.serialize_variant_index(value: 66)
        case .tick(let now):
            try serializer.serialize_variant_index(value: 67)
            try serializer.serialize_i64(value: now)
        case .cooldownClock(let now):
            try serializer.serialize_variant_index(value: 68)
            try serializer.serialize_i64(value: now)
        case .cooldownElapsed:
            try serializer.serialize_variant_index(value: 69)
        case .sessionExpiryStored(let x):
            try serializer.serialize_variant_index(value: 70)
            try x.serialize(serializer: serializer)
        case .offlineSessionChecked(let x):
            try serializer.serialize_variant_index(value: 71)
            try x.serialize(serializer: serializer)
        case .snapshotSaved(let x):
            try serializer.serialize_variant_index(value: 72)
            try x.serialize(serializer: serializer)
        case .snapshotRestored(let x):
            try serializer.serialize_variant_index(value: 73)
            try x.serialize(serializer: serializer)
        case .rotatedTokenStored(let x):
            try serializer.serialize_variant_index(value: 74)
            try x.serialize(serializer: serializer)
        case .attemptRefreshDone:
            try serializer.serialize_variant_index(value: 75)
        case .syncCompleted(let x):
            try serializer.serialize_variant_index(value: 76)
            try x.serialize(serializer: serializer)
        case .requestOtp(let email, let purpose):
            try serializer.serialize_variant_index(value: 77)
            try serializer.serialize_str(value: email)
            try serializer.serialize_str(value: purpose)
        case .otpRequested(let x):
            try serializer.serialize_variant_index(value: 78)
            try x.serialize(serializer: serializer)
        case .verifyOtp(let email, let code, let purpose):
            try serializer.serialize_variant_index(value: 79)
            try serializer.serialize_str(value: email)
            try serializer.serialize_str(value: code)
            try serializer.serialize_str(value: purpose)
        case .otpVerified(let x):
            try serializer.serialize_variant_index(value: 80)
            try x.serialize(serializer: serializer)
        case .register(let email, let password, let otp, let ageConfirmed, let legalAccepted):
            try serializer.serialize_variant_index(value: 81)
            try serializer.serialize_str(value: email)
            try serializer.serialize_str(value: password)
            try serializer.serialize_str(value: otp)
            try serializer.serialize_bool(value: ageConfirmed)
            try serializer.serialize_bool(value: legalAccepted)
        case .registerCompleted(let x):
            try serializer.serialize_variant_index(value: 82)
            try x.serialize(serializer: serializer)
        case .resetPassword(let email, let newPassword, let otp):
            try serializer.serialize_variant_index(value: 83)
            try serializer.serialize_str(value: email)
            try serializer.serialize_str(value: newPassword)
            try serializer.serialize_str(value: otp)
        case .resetPasswordCompleted(let x):
            try serializer.serialize_variant_index(value: 84)
            try x.serialize(serializer: serializer)
        case .queueSavedForSync(let x):
            try serializer.serialize_variant_index(value: 85)
            try x.serialize(serializer: serializer)
        case .startMatch(let nodeId):
            try serializer.serialize_variant_index(value: 86)
            try serializer.serialize_str(value: nodeId)
        case .matchSelectLine(let line):
            try serializer.serialize_variant_index(value: 87)
            try serializer.serialize_i32(value: line)
        case .matchSetAnswer(let answer):
            try serializer.serialize_variant_index(value: 88)
            try serializer.serialize_str(value: answer)
        case .matchSetDropTime(let value):
            try serializer.serialize_variant_index(value: 89)
            try serializer.serialize_str(value: value)
        case .matchSetDropSpace(let value):
            try serializer.serialize_variant_index(value: 90)
            try serializer.serialize_str(value: value)
        case .matchToggleTag(let tag):
            try serializer.serialize_variant_index(value: 91)
            try serializer.serialize_str(value: tag)
        case .matchSetOutput(let value):
            try serializer.serialize_variant_index(value: 92)
            try serializer.serialize_str(value: value)
        case .matchPickTradeoff(let value):
            try serializer.serialize_variant_index(value: 93)
            try serializer.serialize_str(value: value)
        case .matchClearBenefit:
            try serializer.serialize_variant_index(value: 94)
        case .matchClearDrawback:
            try serializer.serialize_variant_index(value: 95)
        case .matchSubmit(let timestamp):
            try serializer.serialize_variant_index(value: 96)
            try serializer.serialize_i64(value: timestamp)
        case .matchDismissTrap:
            try serializer.serialize_variant_index(value: 97)
        case .matchTimerTick:
            try serializer.serialize_variant_index(value: 98)
        case .undoLogout:
            try serializer.serialize_variant_index(value: 99)
        case .dismissLogoutNotice:
            try serializer.serialize_variant_index(value: 100)
        case .logoutUndone(let x):
            try serializer.serialize_variant_index(value: 101)
            try x.serialize(serializer: serializer)
        case .matchAbandonedAt(let solved, let now):
            try serializer.serialize_variant_index(value: 102)
            try serializer.serialize_i32(value: solved)
            try serializer.serialize_i64(value: now)
        case .matchReportClosed:
            try serializer.serialize_variant_index(value: 103)
        case .reviewMilestonesFired(let x):
            try serializer.serialize_variant_index(value: 104)
            try x.serialize(serializer: serializer)
        case .reviewMilestonesRestored(let x):
            try serializer.serialize_variant_index(value: 105)
            try x.serialize(serializer: serializer)
        case .fetchLegalVersions(let country):
            try serializer.serialize_variant_index(value: 106)
            try serializer.serialize_str(value: country)
        case .legalVersionsFetched(let x):
            try serializer.serialize_variant_index(value: 107)
            try x.serialize(serializer: serializer)
        case .deleteAccount(let password):
            try serializer.serialize_variant_index(value: 108)
            try serializer.serialize_str(value: password)
        case .deleteAccountWithProvider(let provider, let idToken, let nonce, let authorizationCode):
            try serializer.serialize_variant_index(value: 109)
            try serializer.serialize_str(value: provider)
            try serializer.serialize_str(value: idToken)
            try serializer.serialize_str(value: nonce)
            try serializer.serialize_str(value: authorizationCode)
        case .deleteAccountWithGitHub(let code, let codeVerifier, let nonce):
            try serializer.serialize_variant_index(value: 110)
            try serializer.serialize_str(value: code)
            try serializer.serialize_str(value: codeVerifier)
            try serializer.serialize_str(value: nonce)
        case .gitHubDeleteExchanged(let x):
            try serializer.serialize_variant_index(value: 111)
            try x.serialize(serializer: serializer)
        case .accountDeleted(let x):
            try serializer.serialize_variant_index(value: 112)
            try x.serialize(serializer: serializer)
        case .dismissDeletionNotice:
            try serializer.serialize_variant_index(value: 113)
        case .dismissAccountRestoredNotice:
            try serializer.serialize_variant_index(value: 114)
        case .setAnalyticsEnabled(let x):
            try serializer.serialize_variant_index(value: 115)
            try serializer.serialize_bool(value: x)
        case .restoreAnalyticsPreference:
            try serializer.serialize_variant_index(value: 116)
        case .analyticsPreferenceRestored(let x):
            try serializer.serialize_variant_index(value: 117)
            try x.serialize(serializer: serializer)
        case .startBoot:
            try serializer.serialize_variant_index(value: 118)
        case .bootWatchdogElapsed(let check, let attempt):
            try serializer.serialize_variant_index(value: 119)
            try check.serialize(serializer: serializer)
            try serializer.serialize_u32(value: attempt)
        case .enterWatchdogElapsed(let attempt):
            try serializer.serialize_variant_index(value: 120)
            try serializer.serialize_u32(value: attempt)
        case .setClientInfo(let appVersion, let platform):
            try serializer.serialize_variant_index(value: 121)
            try serializer.serialize_str(value: appVersion)
            try serializer.serialize_str(value: platform)
        case .fetchTermsPending:
            try serializer.serialize_variant_index(value: 122)
        case .termsPendingFetched(let owner, let result):
            try serializer.serialize_variant_index(value: 123)
            try serializer.serialize_str(value: owner)
            try result.serialize(serializer: serializer)
        case .acceptTerms:
            try serializer.serialize_variant_index(value: 124)
        case .termsAccepted(let x):
            try serializer.serialize_variant_index(value: 125)
            try x.serialize(serializer: serializer)
        case .termsNoticeAccepted(let x):
            try serializer.serialize_variant_index(value: 126)
            try x.serialize(serializer: serializer)
        case .dismissTermsNotice:
            try serializer.serialize_variant_index(value: 127)
        case .retryBoot:
            try serializer.serialize_variant_index(value: 128)
        case .continueOffline:
            try serializer.serialize_variant_index(value: 129)
        case .resumeEmailRead(let x):
            try serializer.serialize_variant_index(value: 130)
            try x.serialize(serializer: serializer)
        case .queueOwnerRead(let x):
            try serializer.serialize_variant_index(value: 131)
            try x.serialize(serializer: serializer)
        case .queueToAdoptRead(let from, let result):
            try serializer.serialize_variant_index(value: 132)
            try serializer.serialize_str(value: from)
            try result.serialize(serializer: serializer)
        case .queueAdopted(let from):
            try serializer.serialize_variant_index(value: 133)
            try serializer.serialize_str(value: from)
        case .syncedQueueRead(let owner, let sent, let result):
            try serializer.serialize_variant_index(value: 134)
            try serializer.serialize_str(value: owner)
            try serializeArray(value: sent, serializer: serializer) { item, serializer in
                try serializer.serialize_str(value: item)
            }
            try result.serialize(serializer: serializer)
        }
        try serializer.decrease_container_depth()
    }

    public func bincodeSerialize() throws -> [UInt8] {
        let serializer = BincodeSerializer.init();
        try self.serialize(serializer: serializer)
        return serializer.get_bytes()
    }

    public static func deserialize<D: Deserializer>(deserializer: D) throws -> Event {
        let index = try deserializer.deserialize_variant_index()
        try deserializer.increase_container_depth()
        switch index {
        case 0:
            let x = try deserializer.deserialize_str()
            try deserializer.decrease_container_depth()
            return .setLocale(x)
        case 1:
            try deserializer.decrease_container_depth()
            return .telemetrySent
        case 2:
            try deserializer.decrease_container_depth()
            return .ping
        case 3:
            try deserializer.decrease_container_depth()
            return .pong
        case 4:
            let email = try deserializer.deserialize_str()
            let password = try deserializer.deserialize_str()
            try deserializer.decrease_container_depth()
            return .login(email: email, password: password)
        case 5:
            let x = try LogN.HttpResult.deserialize(deserializer: deserializer)
            try deserializer.decrease_container_depth()
            return .loginCompleted(x)
        case 6:
            let provider = try deserializer.deserialize_str()
            let idToken = try deserializer.deserialize_str()
            let nonce = try deserializer.deserialize_str()
            try deserializer.decrease_container_depth()
            return .socialLogin(provider: provider, idToken: idToken, nonce: nonce)
        case 7:
            let x = try LogN.HttpResult.deserialize(deserializer: deserializer)
            try deserializer.decrease_container_depth()
            return .socialLoginCompleted(x)
        case 8:
            let ageConfirmed = try deserializer.deserialize_bool()
            let legalAccepted = try deserializer.deserialize_bool()
            try deserializer.decrease_container_depth()
            return .completeSocialSignup(ageConfirmed: ageConfirmed, legalAccepted: legalAccepted)
        case 9:
            try deserializer.decrease_container_depth()
            return .cancelSocialSignup
        case 10:
            try deserializer.decrease_container_depth()
            return .socialLoginFailed
        case 11:
            let code = try deserializer.deserialize_str()
            let codeVerifier = try deserializer.deserialize_str()
            let nonce = try deserializer.deserialize_str()
            try deserializer.decrease_container_depth()
            return .gitHubCodeReceived(code: code, codeVerifier: codeVerifier, nonce: nonce)
        case 12:
            let x = try LogN.HttpResult.deserialize(deserializer: deserializer)
            try deserializer.decrease_container_depth()
            return .gitHubExchanged(x)
        case 13:
            try deserializer.decrease_container_depth()
            return .continueAsGuest
        case 14:
            let x = try LogN.KeyValueResult.deserialize(deserializer: deserializer)
            try deserializer.decrease_container_depth()
            return .tokenStored(x)
        case 15:
            let x = try LogN.KeyValueResult.deserialize(deserializer: deserializer)
            try deserializer.decrease_container_depth()
            return .accountEmailStored(x)
        case 16:
            let x = try LogN.KeyValueResult.deserialize(deserializer: deserializer)
            try deserializer.decrease_container_depth()
            return .accountEmailRead(x)
        case 17:
            try deserializer.decrease_container_depth()
            return .attemptRefresh
        case 18:
            let x = try LogN.KeyValueResult.deserialize(deserializer: deserializer)
            try deserializer.decrease_container_depth()
            return .tokenRead(x)
        case 19:
            let x = try LogN.KeyValueResult.deserialize(deserializer: deserializer)
            try deserializer.decrease_container_depth()
            return .tokenCleared(x)
        case 20:
            let x = try LogN.HttpResult.deserialize(deserializer: deserializer)
            try deserializer.decrease_container_depth()
            return .refreshCompleted(x)
        case 21:
            try deserializer.decrease_container_depth()
            return .logout
        case 22:
            try deserializer.decrease_container_depth()
            return .fetchChallenges
        case 23:
            try deserializer.decrease_container_depth()
            return .fetchNodes
        case 24:
            try deserializer.decrease_container_depth()
            return .fetchProgress
        case 25:
            let x = try LogN.HttpResult.deserialize(deserializer: deserializer)
            try deserializer.decrease_container_depth()
            return .progressFetched(x)
        case 26:
            let x = try LogN.HttpResult.deserialize(deserializer: deserializer)
            try deserializer.decrease_container_depth()
            return .nodesFetched(x)
        case 27:
            let x = try LogN.HttpResult.deserialize(deserializer: deserializer)
            try deserializer.decrease_container_depth()
            return .challengesFetched(x)
        case 28:
            let x = try deserializer.deserialize_str()
            try deserializer.decrease_container_depth()
            return .setDeviceId(x)
        case 29:
            try deserializer.decrease_container_depth()
            return .fetchTracks
        case 30:
            let x = try LogN.HttpResult.deserialize(deserializer: deserializer)
            try deserializer.decrease_container_depth()
            return .tracksFetched(x)
        case 31:
            let jws = try deserializer.deserialize_str()
            let transactionId = try deserializer.deserialize_str()
            let productId = try deserializer.deserialize_str()
            let restore = try deserializer.deserialize_bool()
            let provider = try deserializer.deserialize_str()
            let purchaseToken = try deserializer.deserialize_str()
            try deserializer.decrease_container_depth()
            return .submitPurchase(jws: jws, transactionId: transactionId, productId: productId, restore: restore, provider: provider, purchaseToken: purchaseToken)
        case 32:
            let jws = try deserializer.deserialize_str()
            let transactionId = try deserializer.deserialize_str()
            let productId = try deserializer.deserialize_str()
            let restore = try deserializer.deserialize_bool()
            let provider = try deserializer.deserialize_str()
            let purchaseToken = try deserializer.deserialize_str()
            let result = try LogN.HttpResult.deserialize(deserializer: deserializer)
            try deserializer.decrease_container_depth()
            return .purchaseSubmitted(jws: jws, transactionId: transactionId, productId: productId, restore: restore, provider: provider, purchaseToken: purchaseToken, result: result)
        case 33:
            let transactionId = try deserializer.deserialize_str()
            try deserializer.decrease_container_depth()
            return .purchaseFinished(transactionId: transactionId)
        case 34:
            try deserializer.decrease_container_depth()
            return .closePurchaseFlow
        case 35:
            let productId = try deserializer.deserialize_str()
            try deserializer.decrease_container_depth()
            return .purchaseIntent(productId: productId)
        case 36:
            let count = try deserializer.deserialize_u32()
            try deserializer.decrease_container_depth()
            return .restoreStarted(count: count)
        case 37:
            try deserializer.decrease_container_depth()
            return .dismissRestoreResult
        case 38:
            let trackId = try deserializer.deserialize_str()
            try deserializer.decrease_container_depth()
            return .selectTrack(trackId: trackId)
        case 39:
            let trackId = try deserializer.deserialize_str()
            try deserializer.decrease_container_depth()
            return .dismissSampleOffer(trackId: trackId)
        case 40:
            try deserializer.decrease_container_depth()
            return .catalogOpened
        case 41:
            let seconds = try deserializer.deserialize_i32()
            try deserializer.decrease_container_depth()
            return .setUtcOffset(seconds: seconds)
        case 42:
            try deserializer.decrease_container_depth()
            return .completeOnboarding
        case 43:
            try deserializer.decrease_container_depth()
            return .restorePreferences
        case 44:
            let key = try deserializer.deserialize_str()
            let result = try LogN.KeyValueResult.deserialize(deserializer: deserializer)
            try deserializer.decrease_container_depth()
            return .preferenceRestored(key: key, result: result)
        case 45:
            let trackId = try deserializer.deserialize_str()
            try deserializer.decrease_container_depth()
            return .fetchLicense(trackId: trackId)
        case 46:
            let userId = try deserializer.deserialize_str()
            let trackId = try deserializer.deserialize_str()
            let result = try LogN.HttpResult.deserialize(deserializer: deserializer)
            try deserializer.decrease_container_depth()
            return .licenseFetched(userId: userId, trackId: trackId, result: result)
        case 47:
            let trackId = try deserializer.deserialize_str()
            try deserializer.decrease_container_depth()
            return .fetchPackage(trackId: trackId)
        case 48:
            let userId = try deserializer.deserialize_str()
            let trackId = try deserializer.deserialize_str()
            let result = try LogN.HttpResult.deserialize(deserializer: deserializer)
            try deserializer.decrease_container_depth()
            return .packageFetched(userId: userId, trackId: trackId, result: result)
        case 49:
            let userId = try deserializer.deserialize_str()
            let result = try LogN.KeyValueResult.deserialize(deserializer: deserializer)
            try deserializer.decrease_container_depth()
            return .clockRead(userId: userId, result: result)
        case 50:
            try deserializer.decrease_container_depth()
            return .loadTrackDownloads
        case 51:
            let userId = try deserializer.deserialize_str()
            let result = try LogN.KeyValueResult.deserialize(deserializer: deserializer)
            try deserializer.decrease_container_depth()
            return .trackIndexRead(userId: userId, result: result)
        case 52:
            let userId = try deserializer.deserialize_str()
            let trackId = try deserializer.deserialize_str()
            let result = try LogN.KeyValueResult.deserialize(deserializer: deserializer)
            try deserializer.decrease_container_depth()
            return .licenseRead(userId: userId, trackId: trackId, result: result)
        case 53:
            let userId = try deserializer.deserialize_str()
            let trackId = try deserializer.deserialize_str()
            let result = try LogN.KeyValueResult.deserialize(deserializer: deserializer)
            try deserializer.decrease_container_depth()
            return .packageRead(userId: userId, trackId: trackId, result: result)
        case 54:
            let trackId = try deserializer.deserialize_str()
            try deserializer.decrease_container_depth()
            return .deleteTrackDownload(trackId: trackId)
        case 55:
            try deserializer.decrease_container_depth()
            return .syncNow
        case 56:
            try deserializer.decrease_container_depth()
            return .syncAndLogout
        case 57:
            let owner = try deserializer.deserialize_str()
            let result = try LogN.KeyValueResult.deserialize(deserializer: deserializer)
            try deserializer.decrease_container_depth()
            return .offlineQueueRestored(owner: owner, result: result)
        case 58:
            let json = try deserializer.deserialize_str()
            try deserializer.decrease_container_depth()
            return .bundledTrailLoaded(json: json)
        case 59:
            try deserializer.decrease_container_depth()
            return .leaveMatch
        case 60:
            try deserializer.decrease_container_depth()
            return .confirmLeaveMatch
        case 61:
            try deserializer.decrease_container_depth()
            return .cancelLeaveMatch
        case 62:
            try deserializer.decrease_container_depth()
            return .openOriginSheet
        case 63:
            try deserializer.decrease_container_depth()
            return .closeOriginSheet
        case 64:
            let x = try LogN.KeyValueResult.deserialize(deserializer: deserializer)
            try deserializer.decrease_container_depth()
            return .originsSeenRestored(x)
        case 65:
            let x = try LogN.KeyValueResult.deserialize(deserializer: deserializer)
            try deserializer.decrease_container_depth()
            return .originsSeenStored(x)
        case 66:
            try deserializer.decrease_container_depth()
            return .dismissPasswordReset
        case 67:
            let now = try deserializer.deserialize_i64()
            try deserializer.decrease_container_depth()
            return .tick(now: now)
        case 68:
            let now = try deserializer.deserialize_i64()
            try deserializer.decrease_container_depth()
            return .cooldownClock(now: now)
        case 69:
            try deserializer.decrease_container_depth()
            return .cooldownElapsed
        case 70:
            let x = try LogN.KeyValueResult.deserialize(deserializer: deserializer)
            try deserializer.decrease_container_depth()
            return .sessionExpiryStored(x)
        case 71:
            let x = try LogN.KeyValueResult.deserialize(deserializer: deserializer)
            try deserializer.decrease_container_depth()
            return .offlineSessionChecked(x)
        case 72:
            let x = try LogN.KeyValueResult.deserialize(deserializer: deserializer)
            try deserializer.decrease_container_depth()
            return .snapshotSaved(x)
        case 73:
            let x = try LogN.KeyValueResult.deserialize(deserializer: deserializer)
            try deserializer.decrease_container_depth()
            return .snapshotRestored(x)
        case 74:
            let x = try LogN.KeyValueResult.deserialize(deserializer: deserializer)
            try deserializer.decrease_container_depth()
            return .rotatedTokenStored(x)
        case 75:
            try deserializer.decrease_container_depth()
            return .attemptRefreshDone
        case 76:
            let x = try LogN.HttpResult.deserialize(deserializer: deserializer)
            try deserializer.decrease_container_depth()
            return .syncCompleted(x)
        case 77:
            let email = try deserializer.deserialize_str()
            let purpose = try deserializer.deserialize_str()
            try deserializer.decrease_container_depth()
            return .requestOtp(email: email, purpose: purpose)
        case 78:
            let x = try LogN.HttpResult.deserialize(deserializer: deserializer)
            try deserializer.decrease_container_depth()
            return .otpRequested(x)
        case 79:
            let email = try deserializer.deserialize_str()
            let code = try deserializer.deserialize_str()
            let purpose = try deserializer.deserialize_str()
            try deserializer.decrease_container_depth()
            return .verifyOtp(email: email, code: code, purpose: purpose)
        case 80:
            let x = try LogN.HttpResult.deserialize(deserializer: deserializer)
            try deserializer.decrease_container_depth()
            return .otpVerified(x)
        case 81:
            let email = try deserializer.deserialize_str()
            let password = try deserializer.deserialize_str()
            let otp = try deserializer.deserialize_str()
            let ageConfirmed = try deserializer.deserialize_bool()
            let legalAccepted = try deserializer.deserialize_bool()
            try deserializer.decrease_container_depth()
            return .register(email: email, password: password, otp: otp, ageConfirmed: ageConfirmed, legalAccepted: legalAccepted)
        case 82:
            let x = try LogN.HttpResult.deserialize(deserializer: deserializer)
            try deserializer.decrease_container_depth()
            return .registerCompleted(x)
        case 83:
            let email = try deserializer.deserialize_str()
            let newPassword = try deserializer.deserialize_str()
            let otp = try deserializer.deserialize_str()
            try deserializer.decrease_container_depth()
            return .resetPassword(email: email, newPassword: newPassword, otp: otp)
        case 84:
            let x = try LogN.HttpResult.deserialize(deserializer: deserializer)
            try deserializer.decrease_container_depth()
            return .resetPasswordCompleted(x)
        case 85:
            let x = try LogN.KeyValueResult.deserialize(deserializer: deserializer)
            try deserializer.decrease_container_depth()
            return .queueSavedForSync(x)
        case 86:
            let nodeId = try deserializer.deserialize_str()
            try deserializer.decrease_container_depth()
            return .startMatch(nodeId: nodeId)
        case 87:
            let line = try deserializer.deserialize_i32()
            try deserializer.decrease_container_depth()
            return .matchSelectLine(line: line)
        case 88:
            let answer = try deserializer.deserialize_str()
            try deserializer.decrease_container_depth()
            return .matchSetAnswer(answer: answer)
        case 89:
            let value = try deserializer.deserialize_str()
            try deserializer.decrease_container_depth()
            return .matchSetDropTime(value: value)
        case 90:
            let value = try deserializer.deserialize_str()
            try deserializer.decrease_container_depth()
            return .matchSetDropSpace(value: value)
        case 91:
            let tag = try deserializer.deserialize_str()
            try deserializer.decrease_container_depth()
            return .matchToggleTag(tag: tag)
        case 92:
            let value = try deserializer.deserialize_str()
            try deserializer.decrease_container_depth()
            return .matchSetOutput(value: value)
        case 93:
            let value = try deserializer.deserialize_str()
            try deserializer.decrease_container_depth()
            return .matchPickTradeoff(value: value)
        case 94:
            try deserializer.decrease_container_depth()
            return .matchClearBenefit
        case 95:
            try deserializer.decrease_container_depth()
            return .matchClearDrawback
        case 96:
            let timestamp = try deserializer.deserialize_i64()
            try deserializer.decrease_container_depth()
            return .matchSubmit(timestamp: timestamp)
        case 97:
            try deserializer.decrease_container_depth()
            return .matchDismissTrap
        case 98:
            try deserializer.decrease_container_depth()
            return .matchTimerTick
        case 99:
            try deserializer.decrease_container_depth()
            return .undoLogout
        case 100:
            try deserializer.decrease_container_depth()
            return .dismissLogoutNotice
        case 101:
            let x = try LogN.KeyValueResult.deserialize(deserializer: deserializer)
            try deserializer.decrease_container_depth()
            return .logoutUndone(x)
        case 102:
            let solved = try deserializer.deserialize_i32()
            let now = try deserializer.deserialize_i64()
            try deserializer.decrease_container_depth()
            return .matchAbandonedAt(solved: solved, now: now)
        case 103:
            try deserializer.decrease_container_depth()
            return .matchReportClosed
        case 104:
            let x = try LogN.KeyValueResult.deserialize(deserializer: deserializer)
            try deserializer.decrease_container_depth()
            return .reviewMilestonesFired(x)
        case 105:
            let x = try LogN.KeyValueResult.deserialize(deserializer: deserializer)
            try deserializer.decrease_container_depth()
            return .reviewMilestonesRestored(x)
        case 106:
            let country = try deserializer.deserialize_str()
            try deserializer.decrease_container_depth()
            return .fetchLegalVersions(country: country)
        case 107:
            let x = try LogN.HttpResult.deserialize(deserializer: deserializer)
            try deserializer.decrease_container_depth()
            return .legalVersionsFetched(x)
        case 108:
            let password = try deserializer.deserialize_str()
            try deserializer.decrease_container_depth()
            return .deleteAccount(password: password)
        case 109:
            let provider = try deserializer.deserialize_str()
            let idToken = try deserializer.deserialize_str()
            let nonce = try deserializer.deserialize_str()
            let authorizationCode = try deserializer.deserialize_str()
            try deserializer.decrease_container_depth()
            return .deleteAccountWithProvider(provider: provider, idToken: idToken, nonce: nonce, authorizationCode: authorizationCode)
        case 110:
            let code = try deserializer.deserialize_str()
            let codeVerifier = try deserializer.deserialize_str()
            let nonce = try deserializer.deserialize_str()
            try deserializer.decrease_container_depth()
            return .deleteAccountWithGitHub(code: code, codeVerifier: codeVerifier, nonce: nonce)
        case 111:
            let x = try LogN.HttpResult.deserialize(deserializer: deserializer)
            try deserializer.decrease_container_depth()
            return .gitHubDeleteExchanged(x)
        case 112:
            let x = try LogN.HttpResult.deserialize(deserializer: deserializer)
            try deserializer.decrease_container_depth()
            return .accountDeleted(x)
        case 113:
            try deserializer.decrease_container_depth()
            return .dismissDeletionNotice
        case 114:
            try deserializer.decrease_container_depth()
            return .dismissAccountRestoredNotice
        case 115:
            let x = try deserializer.deserialize_bool()
            try deserializer.decrease_container_depth()
            return .setAnalyticsEnabled(x)
        case 116:
            try deserializer.decrease_container_depth()
            return .restoreAnalyticsPreference
        case 117:
            let x = try LogN.KeyValueResult.deserialize(deserializer: deserializer)
            try deserializer.decrease_container_depth()
            return .analyticsPreferenceRestored(x)
        case 118:
            try deserializer.decrease_container_depth()
            return .startBoot
        case 119:
            let check = try LogN.BootCheck.deserialize(deserializer: deserializer)
            let attempt = try deserializer.deserialize_u32()
            try deserializer.decrease_container_depth()
            return .bootWatchdogElapsed(check: check, attempt: attempt)
        case 120:
            let attempt = try deserializer.deserialize_u32()
            try deserializer.decrease_container_depth()
            return .enterWatchdogElapsed(attempt: attempt)
        case 121:
            let appVersion = try deserializer.deserialize_str()
            let platform = try deserializer.deserialize_str()
            try deserializer.decrease_container_depth()
            return .setClientInfo(appVersion: appVersion, platform: platform)
        case 122:
            try deserializer.decrease_container_depth()
            return .fetchTermsPending
        case 123:
            let owner = try deserializer.deserialize_str()
            let result = try LogN.HttpResult.deserialize(deserializer: deserializer)
            try deserializer.decrease_container_depth()
            return .termsPendingFetched(owner: owner, result: result)
        case 124:
            try deserializer.decrease_container_depth()
            return .acceptTerms
        case 125:
            let x = try LogN.HttpResult.deserialize(deserializer: deserializer)
            try deserializer.decrease_container_depth()
            return .termsAccepted(x)
        case 126:
            let x = try LogN.HttpResult.deserialize(deserializer: deserializer)
            try deserializer.decrease_container_depth()
            return .termsNoticeAccepted(x)
        case 127:
            try deserializer.decrease_container_depth()
            return .dismissTermsNotice
        case 128:
            try deserializer.decrease_container_depth()
            return .retryBoot
        case 129:
            try deserializer.decrease_container_depth()
            return .continueOffline
        case 130:
            let x = try LogN.KeyValueResult.deserialize(deserializer: deserializer)
            try deserializer.decrease_container_depth()
            return .resumeEmailRead(x)
        case 131:
            let x = try LogN.KeyValueResult.deserialize(deserializer: deserializer)
            try deserializer.decrease_container_depth()
            return .queueOwnerRead(x)
        case 132:
            let from = try deserializer.deserialize_str()
            let result = try LogN.KeyValueResult.deserialize(deserializer: deserializer)
            try deserializer.decrease_container_depth()
            return .queueToAdoptRead(from: from, result: result)
        case 133:
            let from = try deserializer.deserialize_str()
            try deserializer.decrease_container_depth()
            return .queueAdopted(from: from)
        case 134:
            let owner = try deserializer.deserialize_str()
            let sent = try deserializeArray(deserializer: deserializer) { deserializer in
                try deserializer.deserialize_str()
            }
            let result = try LogN.KeyValueResult.deserialize(deserializer: deserializer)
            try deserializer.decrease_container_depth()
            return .syncedQueueRead(owner: owner, sent: sent, result: result)
        default: throw DeserializationError.invalidInput(issue: "Unknown variant index for Event: \(index)")
        }
    }

    public static func bincodeDeserialize(input: [UInt8]) throws -> Event {
        let deserializer = BincodeDeserializer.init(input: input);
        let obj = try deserialize(deserializer: deserializer)
        if deserializer.get_buffer_offset() < input.count {
            throw DeserializationError.invalidInput(issue: "Some input bytes were not read")
        }
        return obj
    }
}

/// An error produced when an HTTP request fails.
/// 
/// Variants fall into two groups:
/// 
/// **Transport errors** — generated by the shell when it cannot complete the HTTP
/// exchange. These cross the FFI boundary and are serialized in the protocol:
/// [`Url`](HttpError::Url), [`Io`](HttpError::Io), [`Timeout`](HttpError::Timeout).
/// 
/// **Processing errors** — generated on the Rust side after a response arrives.
/// These are never serialized or visible to shells:
/// 
/// - [`Http`](HttpError::Http) — produced by `Response::new()` when the server returns
/// a 4xx or 5xx status, and **only** then. At the *protocol* level these arrive as
/// [`HttpResult::Ok`](crate::protocol::HttpResult::Ok); `Response::new()` converts
/// them here, so app code using `crux_http::Result<Response<T>>` will see them as
/// `Err(HttpError::Http { code, .. })` — **never** as `Ok(response)` with an error
/// status.
/// - [`Json`](HttpError::Json) — produced when response body deserialisation fails.
/// - [`BodyAlreadyTaken`](HttpError::BodyAlreadyTaken) — a caller took the response body
/// twice.
/// - [`InvalidStatusCode`](HttpError::InvalidStatusCode) — the shell sent a status that
/// isn't valid HTTP.
/// 
/// # Reading what the server said
/// 
/// Because a rejection arrives as an error rather than a response, everything the server
/// sent with it lives on this type: [`HttpError::body`] and [`HttpError::body_json`] for
/// the body, [`HttpError::header`] and [`HttpError::content_type`] for the headers. No
/// matching on the variant's fields required:
/// 
/// ```
/// # use serde::Deserialize;
/// #[derive(Deserialize)]
/// struct ApiError {
/// error: String,
/// }
/// 
/// // the `crux_http::Result` a feature receives when the server rejects a request
/// let result: crux_http::Result<crux_http::Response<Vec<u8>>> =
/// crux_http::testing::rejection(409, r#"{"error":"that overlaps a booked day"}"#);
/// 
/// let error = result.expect_err("a 409 is never Ok");
/// assert_eq!(error.code(), Some(409));
/// assert_eq!(
/// error.body_json::<ApiError>().unwrap().error,
/// "that overlaps a booked day"
/// );
/// ```
indirect public enum HttpError: Hashable, Equatable {
    /// The request URL could not be parsed.
    case url(String)
    /// An IO error prevented the request from completing.
    case io(String)
    /// The request timed out before a response was received.
    case timeout

    public func serialize<S: Serializer>(serializer: S) throws {
        try serializer.increase_container_depth()
        switch self {
        case .url(let x):
            try serializer.serialize_variant_index(value: 0)
            try serializer.serialize_str(value: x)
        case .io(let x):
            try serializer.serialize_variant_index(value: 1)
            try serializer.serialize_str(value: x)
        case .timeout:
            try serializer.serialize_variant_index(value: 2)
        }
        try serializer.decrease_container_depth()
    }

    public func bincodeSerialize() throws -> [UInt8] {
        let serializer = BincodeSerializer.init();
        try self.serialize(serializer: serializer)
        return serializer.get_bytes()
    }

    public static func deserialize<D: Deserializer>(deserializer: D) throws -> HttpError {
        let index = try deserializer.deserialize_variant_index()
        try deserializer.increase_container_depth()
        switch index {
        case 0:
            let x = try deserializer.deserialize_str()
            try deserializer.decrease_container_depth()
            return .url(x)
        case 1:
            let x = try deserializer.deserialize_str()
            try deserializer.decrease_container_depth()
            return .io(x)
        case 2:
            try deserializer.decrease_container_depth()
            return .timeout
        default: throw DeserializationError.invalidInput(issue: "Unknown variant index for HttpError: \(index)")
        }
    }

    public static func bincodeDeserialize(input: [UInt8]) throws -> HttpError {
        let deserializer = BincodeDeserializer.init(input: input);
        let obj = try deserialize(deserializer: deserializer)
        if deserializer.get_buffer_offset() < input.count {
            throw DeserializationError.invalidInput(issue: "Some input bytes were not read")
        }
        return obj
    }
}

public struct HttpHeader: Hashable, Equatable {
    public var name: String
    public var value: String

    public init(name: String, value: String) {
        self.name = name
        self.value = value
    }

    public func serialize<S: Serializer>(serializer: S) throws {
        try serializer.increase_container_depth()
        try serializer.serialize_str(value: self.name)
        try serializer.serialize_str(value: self.value)
        try serializer.decrease_container_depth()
    }

    public func bincodeSerialize() throws -> [UInt8] {
        let serializer = BincodeSerializer.init();
        try self.serialize(serializer: serializer)
        return serializer.get_bytes()
    }

    public static func deserialize<D: Deserializer>(deserializer: D) throws -> HttpHeader {
        try deserializer.increase_container_depth()
        let name = try deserializer.deserialize_str()
        let value = try deserializer.deserialize_str()
        try deserializer.decrease_container_depth()
        return HttpHeader(name: name, value: value)
    }

    public static func bincodeDeserialize(input: [UInt8]) throws -> HttpHeader {
        let deserializer = BincodeDeserializer.init(input: input);
        let obj = try deserialize(deserializer: deserializer)
        if deserializer.get_buffer_offset() < input.count {
            throw DeserializationError.invalidInput(issue: "Some input bytes were not read")
        }
        return obj
    }
}

/// A raw HTTP request, as sent to the shell over the protocol boundary.
/// 
/// # No header validation
/// 
/// All fields are plain strings. Header names and values are carried as-is with no
/// validation against the HTTP specification. This is intentional: `HttpRequest` is a
/// cross-language data carrier deserialised by Swift, Kotlin, and TypeScript shells;
/// Rust's `http`-crate validation rules cannot be enforced on the other side of that
/// boundary.
/// 
/// **Shell authors must not assume that header names or values are well-formed.**
/// Pass them to your underlying HTTP client as-is — it will apply its own rules.
/// 
/// For the ergonomic Rust-side builder that *does* validate header values (and panics
/// on invalid input), see [`crate::command::RequestBuilder`].
public struct HttpRequest: Hashable, Equatable {
    public var method: String
    public var url: String
    public var headers: [HttpHeader]
    public var body: [UInt8]

    public init(method: String, url: String, headers: [HttpHeader], body: [UInt8]) {
        self.method = method
        self.url = url
        self.headers = headers
        self.body = body
    }

    public func serialize<S: Serializer>(serializer: S) throws {
        try serializer.increase_container_depth()
        try serializer.serialize_str(value: self.method)
        try serializer.serialize_str(value: self.url)
        try serializeArray(value: self.headers, serializer: serializer) { item, serializer in
            try item.serialize(serializer: serializer)
        }
        try serializer.serialize_bytes(value: self.body)
        try serializer.decrease_container_depth()
    }

    public func bincodeSerialize() throws -> [UInt8] {
        let serializer = BincodeSerializer.init();
        try self.serialize(serializer: serializer)
        return serializer.get_bytes()
    }

    public static func deserialize<D: Deserializer>(deserializer: D) throws -> HttpRequest {
        try deserializer.increase_container_depth()
        let method = try deserializer.deserialize_str()
        let url = try deserializer.deserialize_str()
        let headers = try deserializeArray(deserializer: deserializer) { deserializer in
            try LogN.HttpHeader.deserialize(deserializer: deserializer)
        }
        let body = try deserializer.deserialize_bytes()
        try deserializer.decrease_container_depth()
        return HttpRequest(method: method, url: url, headers: headers, body: body)
    }

    public static func bincodeDeserialize(input: [UInt8]) throws -> HttpRequest {
        let deserializer = BincodeDeserializer.init(input: input);
        let obj = try deserialize(deserializer: deserializer)
        if deserializer.get_buffer_offset() < input.count {
            throw DeserializationError.invalidInput(issue: "Some input bytes were not read")
        }
        return obj
    }
}

public struct HttpResponse: Hashable, Equatable {
    public var status: UInt16
    public var headers: [HttpHeader]
    public var body: [UInt8]

    public init(status: UInt16, headers: [HttpHeader], body: [UInt8]) {
        self.status = status
        self.headers = headers
        self.body = body
    }

    public func serialize<S: Serializer>(serializer: S) throws {
        try serializer.increase_container_depth()
        try serializer.serialize_u16(value: self.status)
        try serializeArray(value: self.headers, serializer: serializer) { item, serializer in
            try item.serialize(serializer: serializer)
        }
        try serializer.serialize_bytes(value: self.body)
        try serializer.decrease_container_depth()
    }

    public func bincodeSerialize() throws -> [UInt8] {
        let serializer = BincodeSerializer.init();
        try self.serialize(serializer: serializer)
        return serializer.get_bytes()
    }

    public static func deserialize<D: Deserializer>(deserializer: D) throws -> HttpResponse {
        try deserializer.increase_container_depth()
        let status = try deserializer.deserialize_u16()
        let headers = try deserializeArray(deserializer: deserializer) { deserializer in
            try LogN.HttpHeader.deserialize(deserializer: deserializer)
        }
        let body = try deserializer.deserialize_bytes()
        try deserializer.decrease_container_depth()
        return HttpResponse(status: status, headers: headers, body: body)
    }

    public static func bincodeDeserialize(input: [UInt8]) throws -> HttpResponse {
        let deserializer = BincodeDeserializer.init(input: input);
        let obj = try deserialize(deserializer: deserializer)
        if deserializer.get_buffer_offset() < input.count {
            throw DeserializationError.invalidInput(issue: "Some input bytes were not read")
        }
        return obj
    }
}

/// The result of an HTTP request, as returned by the shell over the protocol boundary.
/// 
/// # Status codes are not errors
/// 
/// Any completed HTTP exchange — including responses with 4xx or 5xx status codes — is
/// returned as [`HttpResult::Ok`]. Only *transport-level* failures (the shell could not
/// reach the server at all) produce [`HttpResult::Err`].
/// 
/// To act on an error status, inspect [`HttpResponse::status`]:
/// 
/// ```
/// # use crux_http::protocol::{HttpResult, HttpResponse};
/// # use crux_http::HttpError;
/// # fn handle(result: HttpResult) {
/// match result {
/// HttpResult::Ok(response) if response.status == 200 => { /* success */ }
/// HttpResult::Ok(response) if response.status == 404 => { /* not found */ }
/// HttpResult::Ok(response) if response.status >= 500 => { /* server error */ }
/// HttpResult::Ok(_) => { /* other status */ }
/// HttpResult::Err(e) => { /* transport failure: bad URL, IO error, or timeout */ }
/// }
/// # }
/// ```
indirect public enum HttpResult: Hashable, Equatable {
    /// The shell completed the HTTP exchange. The response may carry any status code,
    /// including 4xx and 5xx — inspect [`HttpResponse::status`] to distinguish them.
    case ok(HttpResponse)
    /// The shell could not complete the HTTP exchange due to a transport-level failure.
    /// See [`HttpError`] for the possible causes.
    case err(HttpError)

    public func serialize<S: Serializer>(serializer: S) throws {
        try serializer.increase_container_depth()
        switch self {
        case .ok(let x):
            try serializer.serialize_variant_index(value: 0)
            try x.serialize(serializer: serializer)
        case .err(let x):
            try serializer.serialize_variant_index(value: 1)
            try x.serialize(serializer: serializer)
        }
        try serializer.decrease_container_depth()
    }

    public func bincodeSerialize() throws -> [UInt8] {
        let serializer = BincodeSerializer.init();
        try self.serialize(serializer: serializer)
        return serializer.get_bytes()
    }

    public static func deserialize<D: Deserializer>(deserializer: D) throws -> HttpResult {
        let index = try deserializer.deserialize_variant_index()
        try deserializer.increase_container_depth()
        switch index {
        case 0:
            let x = try LogN.HttpResponse.deserialize(deserializer: deserializer)
            try deserializer.decrease_container_depth()
            return .ok(x)
        case 1:
            let x = try LogN.HttpError.deserialize(deserializer: deserializer)
            try deserializer.decrease_container_depth()
            return .err(x)
        default: throw DeserializationError.invalidInput(issue: "Unknown variant index for HttpResult: \(index)")
        }
    }

    public static func bincodeDeserialize(input: [UInt8]) throws -> HttpResult {
        let deserializer = BincodeDeserializer.init(input: input);
        let obj = try deserialize(deserializer: deserializer)
        if deserializer.get_buffer_offset() < input.count {
            throw DeserializationError.invalidInput(issue: "Some input bytes were not read")
        }
        return obj
    }
}

/// Represents a point in time (UTC):
/// 
/// - seconds: number of seconds since the Unix epoch (1970-01-01T00:00:00Z)
/// - nanos: number of nanoseconds since the last second
public struct Instant: Hashable, Equatable {
    public var seconds: UInt64
    public var nanos: UInt32

    public init(seconds: UInt64, nanos: UInt32) {
        self.seconds = seconds
        self.nanos = nanos
    }

    public func serialize<S: Serializer>(serializer: S) throws {
        try serializer.increase_container_depth()
        try serializer.serialize_u64(value: self.seconds)
        try serializer.serialize_u32(value: self.nanos)
        try serializer.decrease_container_depth()
    }

    public func bincodeSerialize() throws -> [UInt8] {
        let serializer = BincodeSerializer.init();
        try self.serialize(serializer: serializer)
        return serializer.get_bytes()
    }

    public static func deserialize<D: Deserializer>(deserializer: D) throws -> Instant {
        try deserializer.increase_container_depth()
        let seconds = try deserializer.deserialize_u64()
        let nanos = try deserializer.deserialize_u32()
        try deserializer.decrease_container_depth()
        return Instant(seconds: seconds, nanos: nanos)
    }

    public static func bincodeDeserialize(input: [UInt8]) throws -> Instant {
        let deserializer = BincodeDeserializer.init(input: input);
        let obj = try deserialize(deserializer: deserializer)
        if deserializer.get_buffer_offset() < input.count {
            throw DeserializationError.invalidInput(issue: "Some input bytes were not read")
        }
        return obj
    }
}

/// Error type for `KeyValue` operations
indirect public enum KeyValueError: Hashable, Equatable {
    case io(message: String)
    case timeout
    case cursorNotFound
    case other(message: String)

    public func serialize<S: Serializer>(serializer: S) throws {
        try serializer.increase_container_depth()
        switch self {
        case .io(let message):
            try serializer.serialize_variant_index(value: 0)
            try serializer.serialize_str(value: message)
        case .timeout:
            try serializer.serialize_variant_index(value: 1)
        case .cursorNotFound:
            try serializer.serialize_variant_index(value: 2)
        case .other(let message):
            try serializer.serialize_variant_index(value: 3)
            try serializer.serialize_str(value: message)
        }
        try serializer.decrease_container_depth()
    }

    public func bincodeSerialize() throws -> [UInt8] {
        let serializer = BincodeSerializer.init();
        try self.serialize(serializer: serializer)
        return serializer.get_bytes()
    }

    public static func deserialize<D: Deserializer>(deserializer: D) throws -> KeyValueError {
        let index = try deserializer.deserialize_variant_index()
        try deserializer.increase_container_depth()
        switch index {
        case 0:
            let message = try deserializer.deserialize_str()
            try deserializer.decrease_container_depth()
            return .io(message: message)
        case 1:
            try deserializer.decrease_container_depth()
            return .timeout
        case 2:
            try deserializer.decrease_container_depth()
            return .cursorNotFound
        case 3:
            let message = try deserializer.deserialize_str()
            try deserializer.decrease_container_depth()
            return .other(message: message)
        default: throw DeserializationError.invalidInput(issue: "Unknown variant index for KeyValueError: \(index)")
        }
    }

    public static func bincodeDeserialize(input: [UInt8]) throws -> KeyValueError {
        let deserializer = BincodeDeserializer.init(input: input);
        let obj = try deserialize(deserializer: deserializer)
        if deserializer.get_buffer_offset() < input.count {
            throw DeserializationError.invalidInput(issue: "Some input bytes were not read")
        }
        return obj
    }
}

/// Supported operations
indirect public enum KeyValueOperation: Hashable, Equatable {
    /// Read bytes stored under a key
    case get(key: String)
    /// Write bytes under a key
    case set(key: String, value: [UInt8])
    /// Remove a key and its value
    case delete(key: String)
    /// Test if a key exists
    case exists(key: String)
    case listKeys(prefix: String, cursor: UInt64)

    public func serialize<S: Serializer>(serializer: S) throws {
        try serializer.increase_container_depth()
        switch self {
        case .get(let key):
            try serializer.serialize_variant_index(value: 0)
            try serializer.serialize_str(value: key)
        case .set(let key, let value):
            try serializer.serialize_variant_index(value: 1)
            try serializer.serialize_str(value: key)
            try serializeArray(value: value, serializer: serializer) { item, serializer in
                try serializer.serialize_u8(value: item)
            }
        case .delete(let key):
            try serializer.serialize_variant_index(value: 2)
            try serializer.serialize_str(value: key)
        case .exists(let key):
            try serializer.serialize_variant_index(value: 3)
            try serializer.serialize_str(value: key)
        case .listKeys(let prefix, let cursor):
            try serializer.serialize_variant_index(value: 4)
            try serializer.serialize_str(value: prefix)
            try serializer.serialize_u64(value: cursor)
        }
        try serializer.decrease_container_depth()
    }

    public func bincodeSerialize() throws -> [UInt8] {
        let serializer = BincodeSerializer.init();
        try self.serialize(serializer: serializer)
        return serializer.get_bytes()
    }

    public static func deserialize<D: Deserializer>(deserializer: D) throws -> KeyValueOperation {
        let index = try deserializer.deserialize_variant_index()
        try deserializer.increase_container_depth()
        switch index {
        case 0:
            let key = try deserializer.deserialize_str()
            try deserializer.decrease_container_depth()
            return .get(key: key)
        case 1:
            let key = try deserializer.deserialize_str()
            let value = try deserializeArray(deserializer: deserializer) { deserializer in
                try deserializer.deserialize_u8()
            }
            try deserializer.decrease_container_depth()
            return .set(key: key, value: value)
        case 2:
            let key = try deserializer.deserialize_str()
            try deserializer.decrease_container_depth()
            return .delete(key: key)
        case 3:
            let key = try deserializer.deserialize_str()
            try deserializer.decrease_container_depth()
            return .exists(key: key)
        case 4:
            let prefix = try deserializer.deserialize_str()
            let cursor = try deserializer.deserialize_u64()
            try deserializer.decrease_container_depth()
            return .listKeys(prefix: prefix, cursor: cursor)
        default: throw DeserializationError.invalidInput(issue: "Unknown variant index for KeyValueOperation: \(index)")
        }
    }

    public static func bincodeDeserialize(input: [UInt8]) throws -> KeyValueOperation {
        let deserializer = BincodeDeserializer.init(input: input);
        let obj = try deserialize(deserializer: deserializer)
        if deserializer.get_buffer_offset() < input.count {
            throw DeserializationError.invalidInput(issue: "Some input bytes were not read")
        }
        return obj
    }
}

indirect public enum KeyValueResponse: Hashable, Equatable {
    /// Response to a `KeyValueOperation::Get`,
    /// returning the value stored under the key, which may be empty
    case get(value: Value)
    /// Response to a `KeyValueOperation::Set`,
    /// returning the value that was previously stored under the key, may be empty
    case set(previous: Value)
    /// Response to a `KeyValueOperation::Delete`,
    /// returning the value that was previously stored under the key, may be empty
    case delete(previous: Value)
    /// Response to a `KeyValueOperation::Exists`,
    /// returning whether the key is present in the store
    case exists(isPresent: Bool)
    /// Response to a `KeyValueOperation::ListKeys`,
    /// returning a list of keys that start with the prefix, and a cursor to continue listing
    /// if there are more keys
    /// 
    /// Note: the cursor is 0 if there are no more keys
    case listKeys(keys: [String], nextCursor: UInt64)

    public func serialize<S: Serializer>(serializer: S) throws {
        try serializer.increase_container_depth()
        switch self {
        case .get(let value):
            try serializer.serialize_variant_index(value: 0)
            try value.serialize(serializer: serializer)
        case .set(let previous):
            try serializer.serialize_variant_index(value: 1)
            try previous.serialize(serializer: serializer)
        case .delete(let previous):
            try serializer.serialize_variant_index(value: 2)
            try previous.serialize(serializer: serializer)
        case .exists(let isPresent):
            try serializer.serialize_variant_index(value: 3)
            try serializer.serialize_bool(value: isPresent)
        case .listKeys(let keys, let nextCursor):
            try serializer.serialize_variant_index(value: 4)
            try serializeArray(value: keys, serializer: serializer) { item, serializer in
                try serializer.serialize_str(value: item)
            }
            try serializer.serialize_u64(value: nextCursor)
        }
        try serializer.decrease_container_depth()
    }

    public func bincodeSerialize() throws -> [UInt8] {
        let serializer = BincodeSerializer.init();
        try self.serialize(serializer: serializer)
        return serializer.get_bytes()
    }

    public static func deserialize<D: Deserializer>(deserializer: D) throws -> KeyValueResponse {
        let index = try deserializer.deserialize_variant_index()
        try deserializer.increase_container_depth()
        switch index {
        case 0:
            let value = try LogN.Value.deserialize(deserializer: deserializer)
            try deserializer.decrease_container_depth()
            return .get(value: value)
        case 1:
            let previous = try LogN.Value.deserialize(deserializer: deserializer)
            try deserializer.decrease_container_depth()
            return .set(previous: previous)
        case 2:
            let previous = try LogN.Value.deserialize(deserializer: deserializer)
            try deserializer.decrease_container_depth()
            return .delete(previous: previous)
        case 3:
            let isPresent = try deserializer.deserialize_bool()
            try deserializer.decrease_container_depth()
            return .exists(isPresent: isPresent)
        case 4:
            let keys = try deserializeArray(deserializer: deserializer) { deserializer in
                try deserializer.deserialize_str()
            }
            let nextCursor = try deserializer.deserialize_u64()
            try deserializer.decrease_container_depth()
            return .listKeys(keys: keys, nextCursor: nextCursor)
        default: throw DeserializationError.invalidInput(issue: "Unknown variant index for KeyValueResponse: \(index)")
        }
    }

    public static func bincodeDeserialize(input: [UInt8]) throws -> KeyValueResponse {
        let deserializer = BincodeDeserializer.init(input: input);
        let obj = try deserialize(deserializer: deserializer)
        if deserializer.get_buffer_offset() < input.count {
            throw DeserializationError.invalidInput(issue: "Some input bytes were not read")
        }
        return obj
    }
}

/// The result of an operation on the store.
/// 
/// Note: we can't use [`core::result::Result`] here because it is not currently
/// supported across the FFI boundary, when using `typegen` or `facet_typegen`.
indirect public enum KeyValueResult: Hashable, Equatable {
    case ok(response: KeyValueResponse)
    case err(error: KeyValueError)

    public func serialize<S: Serializer>(serializer: S) throws {
        try serializer.increase_container_depth()
        switch self {
        case .ok(let response):
            try serializer.serialize_variant_index(value: 0)
            try response.serialize(serializer: serializer)
        case .err(let error):
            try serializer.serialize_variant_index(value: 1)
            try error.serialize(serializer: serializer)
        }
        try serializer.decrease_container_depth()
    }

    public func bincodeSerialize() throws -> [UInt8] {
        let serializer = BincodeSerializer.init();
        try self.serialize(serializer: serializer)
        return serializer.get_bytes()
    }

    public static func deserialize<D: Deserializer>(deserializer: D) throws -> KeyValueResult {
        let index = try deserializer.deserialize_variant_index()
        try deserializer.increase_container_depth()
        switch index {
        case 0:
            let response = try LogN.KeyValueResponse.deserialize(deserializer: deserializer)
            try deserializer.decrease_container_depth()
            return .ok(response: response)
        case 1:
            let error = try LogN.KeyValueError.deserialize(deserializer: deserializer)
            try deserializer.decrease_container_depth()
            return .err(error: error)
        default: throw DeserializationError.invalidInput(issue: "Unknown variant index for KeyValueResult: \(index)")
        }
    }

    public static func bincodeDeserialize(input: [UInt8]) throws -> KeyValueResult {
        let deserializer = BincodeDeserializer.init(input: input);
        let obj = try deserialize(deserializer: deserializer)
        if deserializer.get_buffer_offset() < input.count {
            throw DeserializationError.invalidInput(issue: "Some input bytes were not read")
        }
        return obj
    }
}

/// Severidade de um registro de log.
indirect public enum LogLevel: Hashable, Equatable {
    case debug
    case info
    case warn
    case error

    public func serialize<S: Serializer>(serializer: S) throws {
        try serializer.increase_container_depth()
        switch self {
        case .debug:
            try serializer.serialize_variant_index(value: 0)
        case .info:
            try serializer.serialize_variant_index(value: 1)
        case .warn:
            try serializer.serialize_variant_index(value: 2)
        case .error:
            try serializer.serialize_variant_index(value: 3)
        }
        try serializer.decrease_container_depth()
    }

    public func bincodeSerialize() throws -> [UInt8] {
        let serializer = BincodeSerializer.init();
        try self.serialize(serializer: serializer)
        return serializer.get_bytes()
    }

    public static func deserialize<D: Deserializer>(deserializer: D) throws -> LogLevel {
        let index = try deserializer.deserialize_variant_index()
        try deserializer.increase_container_depth()
        switch index {
        case 0:
            try deserializer.decrease_container_depth()
            return .debug
        case 1:
            try deserializer.decrease_container_depth()
            return .info
        case 2:
            try deserializer.decrease_container_depth()
            return .warn
        case 3:
            try deserializer.decrease_container_depth()
            return .error
        default: throw DeserializationError.invalidInput(issue: "Unknown variant index for LogLevel: \(index)")
        }
    }

    public static func bincodeDeserialize(input: [UInt8]) throws -> LogLevel {
        let deserializer = BincodeDeserializer.init(input: input);
        let obj = try deserialize(deserializer: deserializer)
        if deserializer.get_buffer_offset() < input.count {
            throw DeserializationError.invalidInput(issue: "Some input bytes were not read")
        }
        return obj
    }
}

/// Um registro de log estruturado. O shell escolhe o provedor (hoje, os Logs do
/// PostHog). Fire-and-forget, como a telemetria.
/// 
/// Log é o rastro do que aconteceu; o que não deveria acontecer vai também como
/// `MonitoringOperation::LogError`, que vira issue. Mensagem e atributos nunca levam
/// token, senha, OTP, e-mail nem corpo de requisição (AGENTS.md, regra 9): rota,
/// status e código de erro bastam.
public struct LogOperation: Hashable, Equatable {
    public var level: LogLevel
    public var message: String
    public var attributes: [String: String]

    public init(level: LogLevel, message: String, attributes: [String: String]) {
        self.level = level
        self.message = message
        self.attributes = attributes
    }

    public func serialize<S: Serializer>(serializer: S) throws {
        try serializer.increase_container_depth()
        try self.level.serialize(serializer: serializer)
        try serializer.serialize_str(value: self.message)
        try serializeMap(value: self.attributes, serializer: serializer) { key, value, serializer in
            try serializer.serialize_str(value: key)
            try serializer.serialize_str(value: value)
        }
        try serializer.decrease_container_depth()
    }

    public func bincodeSerialize() throws -> [UInt8] {
        let serializer = BincodeSerializer.init();
        try self.serialize(serializer: serializer)
        return serializer.get_bytes()
    }

    public static func deserialize<D: Deserializer>(deserializer: D) throws -> LogOperation {
        try deserializer.increase_container_depth()
        let level = try LogN.LogLevel.deserialize(deserializer: deserializer)
        let message = try deserializer.deserialize_str()
        let attributes = try deserializeMap(deserializer: deserializer) { deserializer in
            let key = try deserializer.deserialize_str()
            let value = try deserializer.deserialize_str()
            return (key, value)
        }
        try deserializer.decrease_container_depth()
        return LogOperation(level: level, message: message, attributes: attributes)
    }

    public static func bincodeDeserialize(input: [UInt8]) throws -> LogOperation {
        let deserializer = BincodeDeserializer.init(input: input);
        let obj = try deserialize(deserializer: deserializer)
        if deserializer.get_buffer_offset() < input.count {
            throw DeserializationError.invalidInput(issue: "Some input bytes were not read")
        }
        return obj
    }
}

/// Um erro da sessão, guardado para a revisão do relatório pós-partida.
/// Um por problema errado — o relatório lista todos, não só o último.
public struct MatchError: Hashable, Equatable {
    public var letter: String
    public var title: String
    /// Sigla do juiz: `WA`, `TLE`, …
    public var verdict: String
    /// O que o jogador respondeu, quando é texto (opção, saída, tags). Vazio no
    /// SPOT_THE_BUG, que usa `given_line`.
    public var givenAnswer: String
    /// Linha escolhida no SPOT_THE_BUG, a partir de 1. -1 quando não se aplica.
    public var givenLine: Int32
    /// A explicação do próprio desafio, crua — pode vir vazia. O cliente completa com a
    /// frase genérica ou com o enquadramento do estouro de tempo, na língua dele.
    public var explanation: String

    public init(letter: String, title: String, verdict: String, givenAnswer: String, givenLine: Int32, explanation: String) {
        self.letter = letter
        self.title = title
        self.verdict = verdict
        self.givenAnswer = givenAnswer
        self.givenLine = givenLine
        self.explanation = explanation
    }

    public func serialize<S: Serializer>(serializer: S) throws {
        try serializer.increase_container_depth()
        try serializer.serialize_str(value: self.letter)
        try serializer.serialize_str(value: self.title)
        try serializer.serialize_str(value: self.verdict)
        try serializer.serialize_str(value: self.givenAnswer)
        try serializer.serialize_i32(value: self.givenLine)
        try serializer.serialize_str(value: self.explanation)
        try serializer.decrease_container_depth()
    }

    public func bincodeSerialize() throws -> [UInt8] {
        let serializer = BincodeSerializer.init();
        try self.serialize(serializer: serializer)
        return serializer.get_bytes()
    }

    public static func deserialize<D: Deserializer>(deserializer: D) throws -> MatchError {
        try deserializer.increase_container_depth()
        let letter = try deserializer.deserialize_str()
        let title = try deserializer.deserialize_str()
        let verdict = try deserializer.deserialize_str()
        let givenAnswer = try deserializer.deserialize_str()
        let givenLine = try deserializer.deserialize_i32()
        let explanation = try deserializer.deserialize_str()
        try deserializer.decrease_container_depth()
        return MatchError(letter: letter, title: title, verdict: verdict, givenAnswer: givenAnswer, givenLine: givenLine, explanation: explanation)
    }

    public static func bincodeDeserialize(input: [UInt8]) throws -> MatchError {
        let deserializer = BincodeDeserializer.init(input: input);
        let obj = try deserialize(deserializer: deserializer)
        if deserializer.get_buffer_offset() < input.count {
            throw DeserializationError.invalidInput(issue: "Some input bytes were not read")
        }
        return obj
    }
}

/// ViewModel da partida para a UI.
public struct MatchViewModel: Hashable, Equatable {
    public var isActive: Bool
    public var currentLetter: String
    public var currentTitle: String
    public var currentDescription: String
    public var currentTemplateType: String
    public var currentCodeLines: [String]
    public var currentOptions: [String]
    public var maxSelections: Int32
    /// Origem do problema atual, vazia quando ele nasceu aqui.
    public var currentOrigin: String
    /// O cartão de origem aberto agora, já com o texto. `None` quando não há cartão na
    /// tela. Quem resolve o id para o cartão, com o fallback de id sem texto conhecido,
    /// é quem monta este ViewModel — ver `to_view_model`.
    public var originSheet: OriginCard?
    /// O cartão aberto está segurando o relógio. Só na primeira leitura de cada origem,
    /// e é isso que o cartão avisa ao jogador.
    public var originSheetPaused: Bool
    /// O cartão de confirmação de saída está na tela.
    public var leavePending: Bool
    /// Quantos problemas do nó já foram aceitos — o que a pessoa deixa para trás se
    /// sair agora, e o número que o cartão mostra.
    public var solvedSoFar: Int32
    public var lives: Int32
    public var maxLives: Int32
    public var penaltyMinutes: Int32
    public var contestSeconds: Int32
    public var questionSeconds: Int32
    public var isFrozen: Bool
    public var totalProblems: Int32
    public var solvedCount: Int32
    public var balloonStates: [BalloonState]
    /// XP que a partida rendeu: só os aceitos que ainda não tinham pago.
    public var xpEarned: Int32
    public var selectedLine: Int32
    public var answerString: String
    public var dropTime: String
    public var dropSpace: String
    public var tradeoffBenefit: String
    public var tradeoffDrawback: String
    public var selectedTags: [String]
    public var predictedOutput: String
    public var watchVariables: [WatchVariable]
    public var watchNote: String
    public var lastVerdict: String
    public var errors: [MatchError]
    public var hasTrap: Bool
    public var trapKind: TrapKind
    public var trapTitle: String
    public var trapExplanation: String

    public init(isActive: Bool, currentLetter: String, currentTitle: String, currentDescription: String, currentTemplateType: String, currentCodeLines: [String], currentOptions: [String], maxSelections: Int32, currentOrigin: String, originSheet: OriginCard?, originSheetPaused: Bool, leavePending: Bool, solvedSoFar: Int32, lives: Int32, maxLives: Int32, penaltyMinutes: Int32, contestSeconds: Int32, questionSeconds: Int32, isFrozen: Bool, totalProblems: Int32, solvedCount: Int32, balloonStates: [BalloonState], xpEarned: Int32, selectedLine: Int32, answerString: String, dropTime: String, dropSpace: String, tradeoffBenefit: String, tradeoffDrawback: String, selectedTags: [String], predictedOutput: String, watchVariables: [WatchVariable], watchNote: String, lastVerdict: String, errors: [MatchError], hasTrap: Bool, trapKind: TrapKind, trapTitle: String, trapExplanation: String) {
        self.isActive = isActive
        self.currentLetter = currentLetter
        self.currentTitle = currentTitle
        self.currentDescription = currentDescription
        self.currentTemplateType = currentTemplateType
        self.currentCodeLines = currentCodeLines
        self.currentOptions = currentOptions
        self.maxSelections = maxSelections
        self.currentOrigin = currentOrigin
        self.originSheet = originSheet
        self.originSheetPaused = originSheetPaused
        self.leavePending = leavePending
        self.solvedSoFar = solvedSoFar
        self.lives = lives
        self.maxLives = maxLives
        self.penaltyMinutes = penaltyMinutes
        self.contestSeconds = contestSeconds
        self.questionSeconds = questionSeconds
        self.isFrozen = isFrozen
        self.totalProblems = totalProblems
        self.solvedCount = solvedCount
        self.balloonStates = balloonStates
        self.xpEarned = xpEarned
        self.selectedLine = selectedLine
        self.answerString = answerString
        self.dropTime = dropTime
        self.dropSpace = dropSpace
        self.tradeoffBenefit = tradeoffBenefit
        self.tradeoffDrawback = tradeoffDrawback
        self.selectedTags = selectedTags
        self.predictedOutput = predictedOutput
        self.watchVariables = watchVariables
        self.watchNote = watchNote
        self.lastVerdict = lastVerdict
        self.errors = errors
        self.hasTrap = hasTrap
        self.trapKind = trapKind
        self.trapTitle = trapTitle
        self.trapExplanation = trapExplanation
    }

    public func serialize<S: Serializer>(serializer: S) throws {
        try serializer.increase_container_depth()
        try serializer.serialize_bool(value: self.isActive)
        try serializer.serialize_str(value: self.currentLetter)
        try serializer.serialize_str(value: self.currentTitle)
        try serializer.serialize_str(value: self.currentDescription)
        try serializer.serialize_str(value: self.currentTemplateType)
        try serializeArray(value: self.currentCodeLines, serializer: serializer) { item, serializer in
            try serializer.serialize_str(value: item)
        }
        try serializeArray(value: self.currentOptions, serializer: serializer) { item, serializer in
            try serializer.serialize_str(value: item)
        }
        try serializer.serialize_i32(value: self.maxSelections)
        try serializer.serialize_str(value: self.currentOrigin)
        try serializeOption(value: self.originSheet, serializer: serializer) { value, serializer in
            try value.serialize(serializer: serializer)
        }
        try serializer.serialize_bool(value: self.originSheetPaused)
        try serializer.serialize_bool(value: self.leavePending)
        try serializer.serialize_i32(value: self.solvedSoFar)
        try serializer.serialize_i32(value: self.lives)
        try serializer.serialize_i32(value: self.maxLives)
        try serializer.serialize_i32(value: self.penaltyMinutes)
        try serializer.serialize_i32(value: self.contestSeconds)
        try serializer.serialize_i32(value: self.questionSeconds)
        try serializer.serialize_bool(value: self.isFrozen)
        try serializer.serialize_i32(value: self.totalProblems)
        try serializer.serialize_i32(value: self.solvedCount)
        try serializeArray(value: self.balloonStates, serializer: serializer) { item, serializer in
            try item.serialize(serializer: serializer)
        }
        try serializer.serialize_i32(value: self.xpEarned)
        try serializer.serialize_i32(value: self.selectedLine)
        try serializer.serialize_str(value: self.answerString)
        try serializer.serialize_str(value: self.dropTime)
        try serializer.serialize_str(value: self.dropSpace)
        try serializer.serialize_str(value: self.tradeoffBenefit)
        try serializer.serialize_str(value: self.tradeoffDrawback)
        try serializeArray(value: self.selectedTags, serializer: serializer) { item, serializer in
            try serializer.serialize_str(value: item)
        }
        try serializer.serialize_str(value: self.predictedOutput)
        try serializeArray(value: self.watchVariables, serializer: serializer) { item, serializer in
            try item.serialize(serializer: serializer)
        }
        try serializer.serialize_str(value: self.watchNote)
        try serializer.serialize_str(value: self.lastVerdict)
        try serializeArray(value: self.errors, serializer: serializer) { item, serializer in
            try item.serialize(serializer: serializer)
        }
        try serializer.serialize_bool(value: self.hasTrap)
        try self.trapKind.serialize(serializer: serializer)
        try serializer.serialize_str(value: self.trapTitle)
        try serializer.serialize_str(value: self.trapExplanation)
        try serializer.decrease_container_depth()
    }

    public func bincodeSerialize() throws -> [UInt8] {
        let serializer = BincodeSerializer.init();
        try self.serialize(serializer: serializer)
        return serializer.get_bytes()
    }

    public static func deserialize<D: Deserializer>(deserializer: D) throws -> MatchViewModel {
        try deserializer.increase_container_depth()
        let isActive = try deserializer.deserialize_bool()
        let currentLetter = try deserializer.deserialize_str()
        let currentTitle = try deserializer.deserialize_str()
        let currentDescription = try deserializer.deserialize_str()
        let currentTemplateType = try deserializer.deserialize_str()
        let currentCodeLines = try deserializeArray(deserializer: deserializer) { deserializer in
            try deserializer.deserialize_str()
        }
        let currentOptions = try deserializeArray(deserializer: deserializer) { deserializer in
            try deserializer.deserialize_str()
        }
        let maxSelections = try deserializer.deserialize_i32()
        let currentOrigin = try deserializer.deserialize_str()
        let originSheet = try deserializeOption(deserializer: deserializer) { deserializer in
            try LogN.OriginCard.deserialize(deserializer: deserializer)
        }
        let originSheetPaused = try deserializer.deserialize_bool()
        let leavePending = try deserializer.deserialize_bool()
        let solvedSoFar = try deserializer.deserialize_i32()
        let lives = try deserializer.deserialize_i32()
        let maxLives = try deserializer.deserialize_i32()
        let penaltyMinutes = try deserializer.deserialize_i32()
        let contestSeconds = try deserializer.deserialize_i32()
        let questionSeconds = try deserializer.deserialize_i32()
        let isFrozen = try deserializer.deserialize_bool()
        let totalProblems = try deserializer.deserialize_i32()
        let solvedCount = try deserializer.deserialize_i32()
        let balloonStates = try deserializeArray(deserializer: deserializer) { deserializer in
            try LogN.BalloonState.deserialize(deserializer: deserializer)
        }
        let xpEarned = try deserializer.deserialize_i32()
        let selectedLine = try deserializer.deserialize_i32()
        let answerString = try deserializer.deserialize_str()
        let dropTime = try deserializer.deserialize_str()
        let dropSpace = try deserializer.deserialize_str()
        let tradeoffBenefit = try deserializer.deserialize_str()
        let tradeoffDrawback = try deserializer.deserialize_str()
        let selectedTags = try deserializeArray(deserializer: deserializer) { deserializer in
            try deserializer.deserialize_str()
        }
        let predictedOutput = try deserializer.deserialize_str()
        let watchVariables = try deserializeArray(deserializer: deserializer) { deserializer in
            try LogN.WatchVariable.deserialize(deserializer: deserializer)
        }
        let watchNote = try deserializer.deserialize_str()
        let lastVerdict = try deserializer.deserialize_str()
        let errors = try deserializeArray(deserializer: deserializer) { deserializer in
            try LogN.MatchError.deserialize(deserializer: deserializer)
        }
        let hasTrap = try deserializer.deserialize_bool()
        let trapKind = try LogN.TrapKind.deserialize(deserializer: deserializer)
        let trapTitle = try deserializer.deserialize_str()
        let trapExplanation = try deserializer.deserialize_str()
        try deserializer.decrease_container_depth()
        return MatchViewModel(isActive: isActive, currentLetter: currentLetter, currentTitle: currentTitle, currentDescription: currentDescription, currentTemplateType: currentTemplateType, currentCodeLines: currentCodeLines, currentOptions: currentOptions, maxSelections: maxSelections, currentOrigin: currentOrigin, originSheet: originSheet, originSheetPaused: originSheetPaused, leavePending: leavePending, solvedSoFar: solvedSoFar, lives: lives, maxLives: maxLives, penaltyMinutes: penaltyMinutes, contestSeconds: contestSeconds, questionSeconds: questionSeconds, isFrozen: isFrozen, totalProblems: totalProblems, solvedCount: solvedCount, balloonStates: balloonStates, xpEarned: xpEarned, selectedLine: selectedLine, answerString: answerString, dropTime: dropTime, dropSpace: dropSpace, tradeoffBenefit: tradeoffBenefit, tradeoffDrawback: tradeoffDrawback, selectedTags: selectedTags, predictedOutput: predictedOutput, watchVariables: watchVariables, watchNote: watchNote, lastVerdict: lastVerdict, errors: errors, hasTrap: hasTrap, trapKind: trapKind, trapTitle: trapTitle, trapExplanation: trapExplanation)
    }

    public static func bincodeDeserialize(input: [UInt8]) throws -> MatchViewModel {
        let deserializer = BincodeDeserializer.init(input: input);
        let obj = try deserialize(deserializer: deserializer)
        if deserializer.get_buffer_offset() < input.count {
            throw DeserializationError.invalidInput(issue: "Some input bytes were not read")
        }
        return obj
    }
}

indirect public enum MonitoringOperation: Hashable, Equatable {
    case logError(message: String, details: String)
    case startSpan(name: String)
    case endSpan(name: String)

    public func serialize<S: Serializer>(serializer: S) throws {
        try serializer.increase_container_depth()
        switch self {
        case .logError(let message, let details):
            try serializer.serialize_variant_index(value: 0)
            try serializer.serialize_str(value: message)
            try serializer.serialize_str(value: details)
        case .startSpan(let name):
            try serializer.serialize_variant_index(value: 1)
            try serializer.serialize_str(value: name)
        case .endSpan(let name):
            try serializer.serialize_variant_index(value: 2)
            try serializer.serialize_str(value: name)
        }
        try serializer.decrease_container_depth()
    }

    public func bincodeSerialize() throws -> [UInt8] {
        let serializer = BincodeSerializer.init();
        try self.serialize(serializer: serializer)
        return serializer.get_bytes()
    }

    public static func deserialize<D: Deserializer>(deserializer: D) throws -> MonitoringOperation {
        let index = try deserializer.deserialize_variant_index()
        try deserializer.increase_container_depth()
        switch index {
        case 0:
            let message = try deserializer.deserialize_str()
            let details = try deserializer.deserialize_str()
            try deserializer.decrease_container_depth()
            return .logError(message: message, details: details)
        case 1:
            let name = try deserializer.deserialize_str()
            try deserializer.decrease_container_depth()
            return .startSpan(name: name)
        case 2:
            let name = try deserializer.deserialize_str()
            try deserializer.decrease_container_depth()
            return .endSpan(name: name)
        default: throw DeserializationError.invalidInput(issue: "Unknown variant index for MonitoringOperation: \(index)")
        }
    }

    public static func bincodeDeserialize(input: [UInt8]) throws -> MonitoringOperation {
        let deserializer = BincodeDeserializer.init(input: input);
        let obj = try deserialize(deserializer: deserializer)
        if deserializer.get_buffer_offset() < input.count {
            throw DeserializationError.invalidInput(issue: "Some input bytes were not read")
        }
        return obj
    }
}

indirect public enum NodeStatus: Hashable, Equatable {
    case locked
    case active
    case completed
    case paywallLocked

    public func serialize<S: Serializer>(serializer: S) throws {
        try serializer.increase_container_depth()
        switch self {
        case .locked:
            try serializer.serialize_variant_index(value: 0)
        case .active:
            try serializer.serialize_variant_index(value: 1)
        case .completed:
            try serializer.serialize_variant_index(value: 2)
        case .paywallLocked:
            try serializer.serialize_variant_index(value: 3)
        }
        try serializer.decrease_container_depth()
    }

    public func bincodeSerialize() throws -> [UInt8] {
        let serializer = BincodeSerializer.init();
        try self.serialize(serializer: serializer)
        return serializer.get_bytes()
    }

    public static func deserialize<D: Deserializer>(deserializer: D) throws -> NodeStatus {
        let index = try deserializer.deserialize_variant_index()
        try deserializer.increase_container_depth()
        switch index {
        case 0:
            try deserializer.decrease_container_depth()
            return .locked
        case 1:
            try deserializer.decrease_container_depth()
            return .active
        case 2:
            try deserializer.decrease_container_depth()
            return .completed
        case 3:
            try deserializer.decrease_container_depth()
            return .paywallLocked
        default: throw DeserializationError.invalidInput(issue: "Unknown variant index for NodeStatus: \(index)")
        }
    }

    public static func bincodeDeserialize(input: [UInt8]) throws -> NodeStatus {
        let deserializer = BincodeDeserializer.init(input: input);
        let obj = try deserialize(deserializer: deserializer)
        if deserializer.get_buffer_offset() < input.count {
            throw DeserializationError.invalidInput(issue: "Some input bytes were not read")
        }
        return obj
    }
}

/// Onde a licença offline de uma trilha comprada está na escada de dias sem contato
/// (LogN Validade Offline). Com rede, cada abertura renova, e só `Silent` aparece.
indirect public enum OfflineState: Hashable, Equatable {
    /// Sem licença no aparelho: não comprada, ou não baixada.
    case noLicense
    /// Mais de 3 dias pela frente. Nada aparece além do "vale até" no detalhe.
    case silent
    /// De 3 a 1 dia: selo no botão Trilhas, card âmbar, bloco no detalhe.
    case soon
    /// Último dia.
    case today
    /// Venceu, ou o relógio foi para antes da emissão. Só esta trilha fecha.
    case expired

    public func serialize<S: Serializer>(serializer: S) throws {
        try serializer.increase_container_depth()
        switch self {
        case .noLicense:
            try serializer.serialize_variant_index(value: 0)
        case .silent:
            try serializer.serialize_variant_index(value: 1)
        case .soon:
            try serializer.serialize_variant_index(value: 2)
        case .today:
            try serializer.serialize_variant_index(value: 3)
        case .expired:
            try serializer.serialize_variant_index(value: 4)
        }
        try serializer.decrease_container_depth()
    }

    public func bincodeSerialize() throws -> [UInt8] {
        let serializer = BincodeSerializer.init();
        try self.serialize(serializer: serializer)
        return serializer.get_bytes()
    }

    public static func deserialize<D: Deserializer>(deserializer: D) throws -> OfflineState {
        let index = try deserializer.deserialize_variant_index()
        try deserializer.increase_container_depth()
        switch index {
        case 0:
            try deserializer.decrease_container_depth()
            return .noLicense
        case 1:
            try deserializer.decrease_container_depth()
            return .silent
        case 2:
            try deserializer.decrease_container_depth()
            return .soon
        case 3:
            try deserializer.decrease_container_depth()
            return .today
        case 4:
            try deserializer.decrease_container_depth()
            return .expired
        default: throw DeserializationError.invalidInput(issue: "Unknown variant index for OfflineState: \(index)")
        }
    }

    public static func bincodeDeserialize(input: [UInt8]) throws -> OfflineState {
        let deserializer = BincodeDeserializer.init(input: input);
        let obj = try deserialize(deserializer: deserializer)
        if deserializer.get_buffer_offset() < input.count {
            throw DeserializationError.invalidInput(issue: "Some input bytes were not read")
        }
        return obj
    }
}

/// O cartão de origem: quem escreveu o desafio, quando não foi escrito para o LogN. O
/// texto vem do servidor pelas tabelas de tradução (ADR 0011), como o resto da trilha —
/// não é mais cópia fixa do catálogo de interface.
public struct OriginCard: Hashable, Equatable {
    public var id: String
    public var name: String
    public var role: String
    public var body: String

    public init(id: String, name: String, role: String, body: String) {
        self.id = id
        self.name = name
        self.role = role
        self.body = body
    }

    public func serialize<S: Serializer>(serializer: S) throws {
        try serializer.increase_container_depth()
        try serializer.serialize_str(value: self.id)
        try serializer.serialize_str(value: self.name)
        try serializer.serialize_str(value: self.role)
        try serializer.serialize_str(value: self.body)
        try serializer.decrease_container_depth()
    }

    public func bincodeSerialize() throws -> [UInt8] {
        let serializer = BincodeSerializer.init();
        try self.serialize(serializer: serializer)
        return serializer.get_bytes()
    }

    public static func deserialize<D: Deserializer>(deserializer: D) throws -> OriginCard {
        try deserializer.increase_container_depth()
        let id = try deserializer.deserialize_str()
        let name = try deserializer.deserialize_str()
        let role = try deserializer.deserialize_str()
        let body = try deserializer.deserialize_str()
        try deserializer.decrease_container_depth()
        return OriginCard(id: id, name: name, role: role, body: body)
    }

    public static func bincodeDeserialize(input: [UInt8]) throws -> OriginCard {
        let deserializer = BincodeDeserializer.init(input: input);
        let obj = try deserialize(deserializer: deserializer)
        if deserializer.get_buffer_offset() < input.count {
            throw DeserializationError.invalidInput(issue: "Some input bytes were not read")
        }
        return obj
    }
}

public struct PurchaseFlowView: Hashable, Equatable {
    public var stage: PurchaseStage
    public var trackId: String
    public var failure: StatusKey

    public init(stage: PurchaseStage, trackId: String, failure: StatusKey) {
        self.stage = stage
        self.trackId = trackId
        self.failure = failure
    }

    public func serialize<S: Serializer>(serializer: S) throws {
        try serializer.increase_container_depth()
        try self.stage.serialize(serializer: serializer)
        try serializer.serialize_str(value: self.trackId)
        try self.failure.serialize(serializer: serializer)
        try serializer.decrease_container_depth()
    }

    public func bincodeSerialize() throws -> [UInt8] {
        let serializer = BincodeSerializer.init();
        try self.serialize(serializer: serializer)
        return serializer.get_bytes()
    }

    public static func deserialize<D: Deserializer>(deserializer: D) throws -> PurchaseFlowView {
        try deserializer.increase_container_depth()
        let stage = try LogN.PurchaseStage.deserialize(deserializer: deserializer)
        let trackId = try deserializer.deserialize_str()
        let failure = try LogN.StatusKey.deserialize(deserializer: deserializer)
        try deserializer.decrease_container_depth()
        return PurchaseFlowView(stage: stage, trackId: trackId, failure: failure)
    }

    public static func bincodeDeserialize(input: [UInt8]) throws -> PurchaseFlowView {
        let deserializer = BincodeDeserializer.init(input: input);
        let obj = try deserialize(deserializer: deserializer)
        if deserializer.get_buffer_offset() < input.count {
            throw DeserializationError.invalidInput(issue: "Some input bytes were not read")
        }
        return obj
    }
}

/// Em que passo está a compra que o jogador acabou de fazer (LogN Trilhas, F3).
indirect public enum PurchaseStage: Hashable, Equatable {
    /// Nenhuma compra na tela.
    case idle
    /// A loja aprovou; o servidor está validando a transação.
    case validating
    /// Transação válida; pedindo a licença.
    case licensing
    /// Licença na mão; baixando o pacote.
    case downloading
    /// Tudo no aparelho: a trilha abre sem rede.
    case ready
    /// Parou; `failure` diz por quê.
    case failed

    public func serialize<S: Serializer>(serializer: S) throws {
        try serializer.increase_container_depth()
        switch self {
        case .idle:
            try serializer.serialize_variant_index(value: 0)
        case .validating:
            try serializer.serialize_variant_index(value: 1)
        case .licensing:
            try serializer.serialize_variant_index(value: 2)
        case .downloading:
            try serializer.serialize_variant_index(value: 3)
        case .ready:
            try serializer.serialize_variant_index(value: 4)
        case .failed:
            try serializer.serialize_variant_index(value: 5)
        }
        try serializer.decrease_container_depth()
    }

    public func bincodeSerialize() throws -> [UInt8] {
        let serializer = BincodeSerializer.init();
        try self.serialize(serializer: serializer)
        return serializer.get_bytes()
    }

    public static func deserialize<D: Deserializer>(deserializer: D) throws -> PurchaseStage {
        let index = try deserializer.deserialize_variant_index()
        try deserializer.increase_container_depth()
        switch index {
        case 0:
            try deserializer.decrease_container_depth()
            return .idle
        case 1:
            try deserializer.decrease_container_depth()
            return .validating
        case 2:
            try deserializer.decrease_container_depth()
            return .licensing
        case 3:
            try deserializer.decrease_container_depth()
            return .downloading
        case 4:
            try deserializer.decrease_container_depth()
            return .ready
        case 5:
            try deserializer.decrease_container_depth()
            return .failed
        default: throw DeserializationError.invalidInput(issue: "Unknown variant index for PurchaseStage: \(index)")
        }
    }

    public static func bincodeDeserialize(input: [UInt8]) throws -> PurchaseStage {
        let deserializer = BincodeDeserializer.init(input: input);
        let obj = try deserialize(deserializer: deserializer)
        if deserializer.get_buffer_offset() < input.count {
            throw DeserializationError.invalidInput(issue: "Some input bytes were not read")
        }
        return obj
    }
}

/// The single operation `Render` implements.
public struct RenderOperation: Hashable, Equatable {
    public init() {
    }

    public func serialize<S: Serializer>(serializer: S) throws {
        try serializer.increase_container_depth()
        try serializer.decrease_container_depth()
    }

    public func bincodeSerialize() throws -> [UInt8] {
        let serializer = BincodeSerializer.init();
        try self.serialize(serializer: serializer)
        return serializer.get_bytes()
    }

    public static func deserialize<D: Deserializer>(deserializer: D) throws -> RenderOperation {
        try deserializer.increase_container_depth()
        try deserializer.decrease_container_depth()
        return RenderOperation()
    }

    public static func bincodeDeserialize(input: [UInt8]) throws -> RenderOperation {
        let deserializer = BincodeDeserializer.init(input: input);
        let obj = try deserialize(deserializer: deserializer)
        if deserializer.get_buffer_offset() < input.count {
            throw DeserializationError.invalidInput(issue: "Some input bytes were not read")
        }
        return obj
    }
}

/// O resultado de "Restaurar compras".
public struct RestoreResultView: Hashable, Equatable {
    /// Há uma restauração na tela, em andamento ou acabada.
    public var active: Bool
    public var finished: Bool
    public var total: UInt32
    public var restored: UInt32
    /// Transações que são de outra conta ativa.
    public var otherAccount: UInt32
    /// Nomes das trilhas que voltaram, na língua do catálogo.
    public var restoredNames: [String]

    public init(active: Bool, finished: Bool, total: UInt32, restored: UInt32, otherAccount: UInt32, restoredNames: [String]) {
        self.active = active
        self.finished = finished
        self.total = total
        self.restored = restored
        self.otherAccount = otherAccount
        self.restoredNames = restoredNames
    }

    public func serialize<S: Serializer>(serializer: S) throws {
        try serializer.increase_container_depth()
        try serializer.serialize_bool(value: self.active)
        try serializer.serialize_bool(value: self.finished)
        try serializer.serialize_u32(value: self.total)
        try serializer.serialize_u32(value: self.restored)
        try serializer.serialize_u32(value: self.otherAccount)
        try serializeArray(value: self.restoredNames, serializer: serializer) { item, serializer in
            try serializer.serialize_str(value: item)
        }
        try serializer.decrease_container_depth()
    }

    public func bincodeSerialize() throws -> [UInt8] {
        let serializer = BincodeSerializer.init();
        try self.serialize(serializer: serializer)
        return serializer.get_bytes()
    }

    public static func deserialize<D: Deserializer>(deserializer: D) throws -> RestoreResultView {
        try deserializer.increase_container_depth()
        let active = try deserializer.deserialize_bool()
        let finished = try deserializer.deserialize_bool()
        let total = try deserializer.deserialize_u32()
        let restored = try deserializer.deserialize_u32()
        let otherAccount = try deserializer.deserialize_u32()
        let restoredNames = try deserializeArray(deserializer: deserializer) { deserializer in
            try deserializer.deserialize_str()
        }
        try deserializer.decrease_container_depth()
        return RestoreResultView(active: active, finished: finished, total: total, restored: restored, otherAccount: otherAccount, restoredNames: restoredNames)
    }

    public static func bincodeDeserialize(input: [UInt8]) throws -> RestoreResultView {
        let deserializer = BincodeDeserializer.init(input: input);
        let obj = try deserialize(deserializer: deserializer)
        if deserializer.get_buffer_offset() < input.count {
            throw DeserializationError.invalidInput(issue: "Some input bytes were not read")
        }
        return obj
    }
}

/// A oferta do fim da amostra, no relatório da partida (LogN Trilhas, 1f).
public struct SampleOfferView: Hashable, Equatable {
    public var active: Bool
    public var trackId: String
    public var nextNodeName: String
    /// Posição do próximo nó na trilha, a partir de 1.
    public var nextNodeIndex: UInt32
    public var remainingNodes: UInt32
    public var remainingProblems: UInt32

    public init(active: Bool, trackId: String, nextNodeName: String, nextNodeIndex: UInt32, remainingNodes: UInt32, remainingProblems: UInt32) {
        self.active = active
        self.trackId = trackId
        self.nextNodeName = nextNodeName
        self.nextNodeIndex = nextNodeIndex
        self.remainingNodes = remainingNodes
        self.remainingProblems = remainingProblems
    }

    public func serialize<S: Serializer>(serializer: S) throws {
        try serializer.increase_container_depth()
        try serializer.serialize_bool(value: self.active)
        try serializer.serialize_str(value: self.trackId)
        try serializer.serialize_str(value: self.nextNodeName)
        try serializer.serialize_u32(value: self.nextNodeIndex)
        try serializer.serialize_u32(value: self.remainingNodes)
        try serializer.serialize_u32(value: self.remainingProblems)
        try serializer.decrease_container_depth()
    }

    public func bincodeSerialize() throws -> [UInt8] {
        let serializer = BincodeSerializer.init();
        try self.serialize(serializer: serializer)
        return serializer.get_bytes()
    }

    public static func deserialize<D: Deserializer>(deserializer: D) throws -> SampleOfferView {
        try deserializer.increase_container_depth()
        let active = try deserializer.deserialize_bool()
        let trackId = try deserializer.deserialize_str()
        let nextNodeName = try deserializer.deserialize_str()
        let nextNodeIndex = try deserializer.deserialize_u32()
        let remainingNodes = try deserializer.deserialize_u32()
        let remainingProblems = try deserializer.deserialize_u32()
        try deserializer.decrease_container_depth()
        return SampleOfferView(active: active, trackId: trackId, nextNodeName: nextNodeName, nextNodeIndex: nextNodeIndex, remainingNodes: remainingNodes, remainingProblems: remainingProblems)
    }

    public static func bincodeDeserialize(input: [UInt8]) throws -> SampleOfferView {
        let deserializer = BincodeDeserializer.init(input: input);
        let obj = try deserialize(deserializer: deserializer)
        if deserializer.get_buffer_offset() < input.count {
            throw DeserializationError.invalidInput(issue: "Some input bytes were not read")
        }
        return obj
    }
}

public struct ScoreCell: Hashable, Equatable {
    public var state: ScoreCellState
    public var top: String
    public var bottom: String

    public init(state: ScoreCellState, top: String, bottom: String) {
        self.state = state
        self.top = top
        self.bottom = bottom
    }

    public func serialize<S: Serializer>(serializer: S) throws {
        try serializer.increase_container_depth()
        try self.state.serialize(serializer: serializer)
        try serializer.serialize_str(value: self.top)
        try serializer.serialize_str(value: self.bottom)
        try serializer.decrease_container_depth()
    }

    public func bincodeSerialize() throws -> [UInt8] {
        let serializer = BincodeSerializer.init();
        try self.serialize(serializer: serializer)
        return serializer.get_bytes()
    }

    public static func deserialize<D: Deserializer>(deserializer: D) throws -> ScoreCell {
        try deserializer.increase_container_depth()
        let state = try LogN.ScoreCellState.deserialize(deserializer: deserializer)
        let top = try deserializer.deserialize_str()
        let bottom = try deserializer.deserialize_str()
        try deserializer.decrease_container_depth()
        return ScoreCell(state: state, top: top, bottom: bottom)
    }

    public static func bincodeDeserialize(input: [UInt8]) throws -> ScoreCell {
        let deserializer = BincodeDeserializer.init(input: input);
        let obj = try deserialize(deserializer: deserializer)
        if deserializer.get_buffer_offset() < input.count {
            throw DeserializationError.invalidInput(issue: "Some input bytes were not read")
        }
        return obj
    }
}

/// Estado de uma célula do telão. Os quatro estados do DS, e só eles.
indirect public enum ScoreCellState: Hashable, Equatable {
    /// Aceito — `+` ou `+N` em cima, minuto do AC embaixo.
    case accepted
    /// Tentado sem AC — `−N` em cima.
    case failed
    /// Submetido após o congelamento — `?` em cima, `frz` embaixo.
    case frozen
    /// Não tentado — célula vazia.
    case untried

    public func serialize<S: Serializer>(serializer: S) throws {
        try serializer.increase_container_depth()
        switch self {
        case .accepted:
            try serializer.serialize_variant_index(value: 0)
        case .failed:
            try serializer.serialize_variant_index(value: 1)
        case .frozen:
            try serializer.serialize_variant_index(value: 2)
        case .untried:
            try serializer.serialize_variant_index(value: 3)
        }
        try serializer.decrease_container_depth()
    }

    public func bincodeSerialize() throws -> [UInt8] {
        let serializer = BincodeSerializer.init();
        try self.serialize(serializer: serializer)
        return serializer.get_bytes()
    }

    public static func deserialize<D: Deserializer>(deserializer: D) throws -> ScoreCellState {
        let index = try deserializer.deserialize_variant_index()
        try deserializer.increase_container_depth()
        switch index {
        case 0:
            try deserializer.decrease_container_depth()
            return .accepted
        case 1:
            try deserializer.decrease_container_depth()
            return .failed
        case 2:
            try deserializer.decrease_container_depth()
            return .frozen
        case 3:
            try deserializer.decrease_container_depth()
            return .untried
        default: throw DeserializationError.invalidInput(issue: "Unknown variant index for ScoreCellState: \(index)")
        }
    }

    public static func bincodeDeserialize(input: [UInt8]) throws -> ScoreCellState {
        let deserializer = BincodeDeserializer.init(input: input);
        let obj = try deserialize(deserializer: deserializer)
        if deserializer.get_buffer_offset() < input.count {
            throw DeserializationError.invalidInput(issue: "Some input bytes were not read")
        }
        return obj
    }
}

/// Uma linha do telão. 13 células, uma por letra A—M.
public struct ScoreboardRow: Hashable, Equatable {
    public var rank: Int32
    public var team: String
    public var university: String
    public var solved: Int32
    public var penalty: Int32
    public var isUser: Bool
    public var cells: [ScoreCell]

    public init(rank: Int32, team: String, university: String, solved: Int32, penalty: Int32, isUser: Bool, cells: [ScoreCell]) {
        self.rank = rank
        self.team = team
        self.university = university
        self.solved = solved
        self.penalty = penalty
        self.isUser = isUser
        self.cells = cells
    }

    public func serialize<S: Serializer>(serializer: S) throws {
        try serializer.increase_container_depth()
        try serializer.serialize_i32(value: self.rank)
        try serializer.serialize_str(value: self.team)
        try serializer.serialize_str(value: self.university)
        try serializer.serialize_i32(value: self.solved)
        try serializer.serialize_i32(value: self.penalty)
        try serializer.serialize_bool(value: self.isUser)
        try serializeArray(value: self.cells, serializer: serializer) { item, serializer in
            try item.serialize(serializer: serializer)
        }
        try serializer.decrease_container_depth()
    }

    public func bincodeSerialize() throws -> [UInt8] {
        let serializer = BincodeSerializer.init();
        try self.serialize(serializer: serializer)
        return serializer.get_bytes()
    }

    public static func deserialize<D: Deserializer>(deserializer: D) throws -> ScoreboardRow {
        try deserializer.increase_container_depth()
        let rank = try deserializer.deserialize_i32()
        let team = try deserializer.deserialize_str()
        let university = try deserializer.deserialize_str()
        let solved = try deserializer.deserialize_i32()
        let penalty = try deserializer.deserialize_i32()
        let isUser = try deserializer.deserialize_bool()
        let cells = try deserializeArray(deserializer: deserializer) { deserializer in
            try LogN.ScoreCell.deserialize(deserializer: deserializer)
        }
        try deserializer.decrease_container_depth()
        return ScoreboardRow(rank: rank, team: team, university: university, solved: solved, penalty: penalty, isUser: isUser, cells: cells)
    }

    public static func bincodeDeserialize(input: [UInt8]) throws -> ScoreboardRow {
        let deserializer = BincodeDeserializer.init(input: input);
        let obj = try deserialize(deserializer: deserializer)
        if deserializer.get_buffer_offset() < input.count {
            throw DeserializationError.invalidInput(issue: "Some input bytes were not read")
        }
        return obj
    }
}

public struct SkillNode: Hashable, Equatable {
    public var id: String
    public var name: String
    public var description: String
    public var row: Int32
    public var column: Int32
    public var requiredXp: Int32
    public var prerequisites: [String]
    public var trackId: String
    /// Nó de trilha paga fora da amostra: só abre com licença. Quem decide é o
    /// servidor; o Core não adivinha pela linha nem por um id de trilha fixo.
    public var requiresPurchase: Bool
    /// Assunto do nó, neutro (`adhoc`, `graphs`): decide cor e ícone no cliente, que
    /// antes adivinhava pelo nome — e o nome agora muda com a língua. Vazio quando o
    /// servidor ou o retrato antigo não mandam.
    public var topic: String
    /// Derivado em `view()` a partir do XP e do DAG, nunca persistido: o Go não manda
    /// este campo, e sem o `default` a lista inteira falhava no `serde` — o usuário
    /// autenticado via uma árvore vazia sem erro nenhum aparecer.
    public var status: NodeStatus
    /// Um por problema do nó, na ordem das letras: `true` quando ele já rendeu XP.
    /// Derivado em `view()`, como `status`. É o que a trilha conta (`3/5`) e o que o
    /// sheet desenha balão a balão.
    public var problemsSolved: [Bool]

    public init(id: String, name: String, description: String, row: Int32, column: Int32, requiredXp: Int32, prerequisites: [String], trackId: String, requiresPurchase: Bool, topic: String, status: NodeStatus, problemsSolved: [Bool]) {
        self.id = id
        self.name = name
        self.description = description
        self.row = row
        self.column = column
        self.requiredXp = requiredXp
        self.prerequisites = prerequisites
        self.trackId = trackId
        self.requiresPurchase = requiresPurchase
        self.topic = topic
        self.status = status
        self.problemsSolved = problemsSolved
    }

    public func serialize<S: Serializer>(serializer: S) throws {
        try serializer.increase_container_depth()
        try serializer.serialize_str(value: self.id)
        try serializer.serialize_str(value: self.name)
        try serializer.serialize_str(value: self.description)
        try serializer.serialize_i32(value: self.row)
        try serializer.serialize_i32(value: self.column)
        try serializer.serialize_i32(value: self.requiredXp)
        try serializeArray(value: self.prerequisites, serializer: serializer) { item, serializer in
            try serializer.serialize_str(value: item)
        }
        try serializer.serialize_str(value: self.trackId)
        try serializer.serialize_bool(value: self.requiresPurchase)
        try serializer.serialize_str(value: self.topic)
        try self.status.serialize(serializer: serializer)
        try serializeArray(value: self.problemsSolved, serializer: serializer) { item, serializer in
            try serializer.serialize_bool(value: item)
        }
        try serializer.decrease_container_depth()
    }

    public func bincodeSerialize() throws -> [UInt8] {
        let serializer = BincodeSerializer.init();
        try self.serialize(serializer: serializer)
        return serializer.get_bytes()
    }

    public static func deserialize<D: Deserializer>(deserializer: D) throws -> SkillNode {
        try deserializer.increase_container_depth()
        let id = try deserializer.deserialize_str()
        let name = try deserializer.deserialize_str()
        let description = try deserializer.deserialize_str()
        let row = try deserializer.deserialize_i32()
        let column = try deserializer.deserialize_i32()
        let requiredXp = try deserializer.deserialize_i32()
        let prerequisites = try deserializeArray(deserializer: deserializer) { deserializer in
            try deserializer.deserialize_str()
        }
        let trackId = try deserializer.deserialize_str()
        let requiresPurchase = try deserializer.deserialize_bool()
        let topic = try deserializer.deserialize_str()
        let status = try LogN.NodeStatus.deserialize(deserializer: deserializer)
        let problemsSolved = try deserializeArray(deserializer: deserializer) { deserializer in
            try deserializer.deserialize_bool()
        }
        try deserializer.decrease_container_depth()
        return SkillNode(id: id, name: name, description: description, row: row, column: column, requiredXp: requiredXp, prerequisites: prerequisites, trackId: trackId, requiresPurchase: requiresPurchase, topic: topic, status: status, problemsSolved: problemsSolved)
    }

    public static func bincodeDeserialize(input: [UInt8]) throws -> SkillNode {
        let deserializer = BincodeDeserializer.init(input: input);
        let obj = try deserialize(deserializer: deserializer)
        if deserializer.get_buffer_offset() < input.count {
            throw DeserializationError.invalidInput(issue: "Some input bytes were not read")
        }
        return obj
    }
}

/// Uma linha do ranking de celular.
public struct StandingRow: Hashable, Equatable {
    public var rank: Int32
    public var handle: String
    public var university: String
    public var solved: Int32
    public var penalty: Int32
    public var isUser: Bool
    /// Só na linha do usuário: `subiu 6 nesta rodada`. Vazio nas demais.
    public var note: String

    public init(rank: Int32, handle: String, university: String, solved: Int32, penalty: Int32, isUser: Bool, note: String) {
        self.rank = rank
        self.handle = handle
        self.university = university
        self.solved = solved
        self.penalty = penalty
        self.isUser = isUser
        self.note = note
    }

    public func serialize<S: Serializer>(serializer: S) throws {
        try serializer.increase_container_depth()
        try serializer.serialize_i32(value: self.rank)
        try serializer.serialize_str(value: self.handle)
        try serializer.serialize_str(value: self.university)
        try serializer.serialize_i32(value: self.solved)
        try serializer.serialize_i32(value: self.penalty)
        try serializer.serialize_bool(value: self.isUser)
        try serializer.serialize_str(value: self.note)
        try serializer.decrease_container_depth()
    }

    public func bincodeSerialize() throws -> [UInt8] {
        let serializer = BincodeSerializer.init();
        try self.serialize(serializer: serializer)
        return serializer.get_bytes()
    }

    public static func deserialize<D: Deserializer>(deserializer: D) throws -> StandingRow {
        try deserializer.increase_container_depth()
        let rank = try deserializer.deserialize_i32()
        let handle = try deserializer.deserialize_str()
        let university = try deserializer.deserialize_str()
        let solved = try deserializer.deserialize_i32()
        let penalty = try deserializer.deserialize_i32()
        let isUser = try deserializer.deserialize_bool()
        let note = try deserializer.deserialize_str()
        try deserializer.decrease_container_depth()
        return StandingRow(rank: rank, handle: handle, university: university, solved: solved, penalty: penalty, isUser: isUser, note: note)
    }

    public static func bincodeDeserialize(input: [UInt8]) throws -> StandingRow {
        let deserializer = BincodeDeserializer.init(input: input);
        let obj = try deserialize(deserializer: deserializer)
        if deserializer.get_buffer_offset() < input.count {
            throw DeserializationError.invalidInput(issue: "Some input bytes were not read")
        }
        return obj
    }
}

/// O que o Core tem a dizer ao jogador, como **chave**, não como frase.
/// 
/// O Core escrevia a frase pronta em `status`, e ela vazava para a tela: a de login
/// abria com "Logged out successfully", em inglês, no vermelho de erro, num app em
/// português. A cópia vive em `i18n/locales/`; o cliente resolve a chave.
/// 
/// `Silent` é o estado normal — a maior parte do que acontece não precisa ser narrada.
indirect public enum StatusKey: Hashable, Equatable {
    case silent
    case signingIn
    case wrongCredentials
    case signInFailed
    case noConnection
    case serverUnreadable
    case sessionExpired
    case signingOut
    case resumingSession
    case sendingCode
    case codeSentFailed
    case checkingCode
    case codeInvalid
    case creatingAccount
    case accountFailed
    case resettingPassword
    case resetFailed
    case syncing
    case syncDiverged
    case syncFailed
    case syncOffline
    case signInToSync
    case treeUnavailable
    /// O servidor respondeu 429. Os botões que batem nele ficam travados pela
    /// contagem do `Retry-After`, e a própria contagem aparece neles.
    case rateLimited
    /// Os quatro abaixo vêm do código de erro da API (`invalid_email`,
    /// `password_too_short`, `password_too_long`, `email_taken`). Antes todos viravam
    /// "não deu para criar a conta", sem dizer o que corrigir.
    case invalidEmail
    case passwordTooShort
    case passwordTooLong
    case emailTaken
    /// A compra foi feita na loja e o servidor está confirmando.
    case purchaseConfirming
    case purchaseConfirmed
    /// O servidor não confirmou agora (rede, erro dele). A loja entrega de novo na
    /// próxima abertura, e o app tenta outra vez sozinho.
    case purchaseFailed
    /// `purchase_owned_by_other_account`: a compra é de outra conta ativa.
    case purchaseOwnedByOtherAccount
    /// `purchase_account_mismatch`: a compra foi feita logado em outra conta.
    case purchaseAccountMismatch
    /// `purchase_revoked`: a compra foi reembolsada ou revogada.
    case purchaseRevoked
    /// Comprar pede conta; o visitante não compra.
    case purchaseNeedsAccount
    /// A licença de uma trilha baixada foi revogada: chave e pacote saíram do aparelho.
    case trackRevoked
    /// A compra está na conta, mas a licença ou o pacote não chegaram agora. Baixa
    /// sozinha na próxima abertura com rede.
    case trackDownloadFailed
    /// O servidor recusou o token do provedor (`social_token_invalid`), ou o login
    /// nele não chegou ao fim.
    case socialSignInFailed
    /// `social_email_unverified`: o provedor não confirmou o e-mail da conta.
    case socialEmailUnverified
    /// `provider_disabled`: o servidor está com o login por esse provedor desligado.
    case socialProviderDisabled
    /// `provider_reauth_required`: excluir a conta pede a confirmação pela Apple, não a
    /// senha (ADR 0017).
    case providerReauthRequired
    /// `purchase_pending`: o Google Play aceitou a compra, mas o pagamento ainda não caiu
    /// (boleto, dinheiro). A trilha abre quando cair.
    case purchasePending
    /// `store_unavailable`: o servidor não fala com a loja agora. Tentar de novo depois.
    case storeUnavailable
    /// `login_locked`: senha errada demais para este e-mail. Trava só o login por senha;
    /// o login social e a troca de senha pelo código continuam abertos, e o app não
    /// trava botão.
    case loginLocked
    /// `otp_locked`: código errado demais para este e-mail. Nenhum código novo sai até a
    /// janela do servidor vencer (um dia), e esperar um minuto não resolve.
    case codeLocked

    public func serialize<S: Serializer>(serializer: S) throws {
        try serializer.increase_container_depth()
        switch self {
        case .silent:
            try serializer.serialize_variant_index(value: 0)
        case .signingIn:
            try serializer.serialize_variant_index(value: 1)
        case .wrongCredentials:
            try serializer.serialize_variant_index(value: 2)
        case .signInFailed:
            try serializer.serialize_variant_index(value: 3)
        case .noConnection:
            try serializer.serialize_variant_index(value: 4)
        case .serverUnreadable:
            try serializer.serialize_variant_index(value: 5)
        case .sessionExpired:
            try serializer.serialize_variant_index(value: 6)
        case .signingOut:
            try serializer.serialize_variant_index(value: 7)
        case .resumingSession:
            try serializer.serialize_variant_index(value: 8)
        case .sendingCode:
            try serializer.serialize_variant_index(value: 9)
        case .codeSentFailed:
            try serializer.serialize_variant_index(value: 10)
        case .checkingCode:
            try serializer.serialize_variant_index(value: 11)
        case .codeInvalid:
            try serializer.serialize_variant_index(value: 12)
        case .creatingAccount:
            try serializer.serialize_variant_index(value: 13)
        case .accountFailed:
            try serializer.serialize_variant_index(value: 14)
        case .resettingPassword:
            try serializer.serialize_variant_index(value: 15)
        case .resetFailed:
            try serializer.serialize_variant_index(value: 16)
        case .syncing:
            try serializer.serialize_variant_index(value: 17)
        case .syncDiverged:
            try serializer.serialize_variant_index(value: 18)
        case .syncFailed:
            try serializer.serialize_variant_index(value: 19)
        case .syncOffline:
            try serializer.serialize_variant_index(value: 20)
        case .signInToSync:
            try serializer.serialize_variant_index(value: 21)
        case .treeUnavailable:
            try serializer.serialize_variant_index(value: 22)
        case .rateLimited:
            try serializer.serialize_variant_index(value: 23)
        case .invalidEmail:
            try serializer.serialize_variant_index(value: 24)
        case .passwordTooShort:
            try serializer.serialize_variant_index(value: 25)
        case .passwordTooLong:
            try serializer.serialize_variant_index(value: 26)
        case .emailTaken:
            try serializer.serialize_variant_index(value: 27)
        case .purchaseConfirming:
            try serializer.serialize_variant_index(value: 28)
        case .purchaseConfirmed:
            try serializer.serialize_variant_index(value: 29)
        case .purchaseFailed:
            try serializer.serialize_variant_index(value: 30)
        case .purchaseOwnedByOtherAccount:
            try serializer.serialize_variant_index(value: 31)
        case .purchaseAccountMismatch:
            try serializer.serialize_variant_index(value: 32)
        case .purchaseRevoked:
            try serializer.serialize_variant_index(value: 33)
        case .purchaseNeedsAccount:
            try serializer.serialize_variant_index(value: 34)
        case .trackRevoked:
            try serializer.serialize_variant_index(value: 35)
        case .trackDownloadFailed:
            try serializer.serialize_variant_index(value: 36)
        case .socialSignInFailed:
            try serializer.serialize_variant_index(value: 37)
        case .socialEmailUnverified:
            try serializer.serialize_variant_index(value: 38)
        case .socialProviderDisabled:
            try serializer.serialize_variant_index(value: 39)
        case .providerReauthRequired:
            try serializer.serialize_variant_index(value: 40)
        case .purchasePending:
            try serializer.serialize_variant_index(value: 41)
        case .storeUnavailable:
            try serializer.serialize_variant_index(value: 42)
        case .loginLocked:
            try serializer.serialize_variant_index(value: 43)
        case .codeLocked:
            try serializer.serialize_variant_index(value: 44)
        }
        try serializer.decrease_container_depth()
    }

    public func bincodeSerialize() throws -> [UInt8] {
        let serializer = BincodeSerializer.init();
        try self.serialize(serializer: serializer)
        return serializer.get_bytes()
    }

    public static func deserialize<D: Deserializer>(deserializer: D) throws -> StatusKey {
        let index = try deserializer.deserialize_variant_index()
        try deserializer.increase_container_depth()
        switch index {
        case 0:
            try deserializer.decrease_container_depth()
            return .silent
        case 1:
            try deserializer.decrease_container_depth()
            return .signingIn
        case 2:
            try deserializer.decrease_container_depth()
            return .wrongCredentials
        case 3:
            try deserializer.decrease_container_depth()
            return .signInFailed
        case 4:
            try deserializer.decrease_container_depth()
            return .noConnection
        case 5:
            try deserializer.decrease_container_depth()
            return .serverUnreadable
        case 6:
            try deserializer.decrease_container_depth()
            return .sessionExpired
        case 7:
            try deserializer.decrease_container_depth()
            return .signingOut
        case 8:
            try deserializer.decrease_container_depth()
            return .resumingSession
        case 9:
            try deserializer.decrease_container_depth()
            return .sendingCode
        case 10:
            try deserializer.decrease_container_depth()
            return .codeSentFailed
        case 11:
            try deserializer.decrease_container_depth()
            return .checkingCode
        case 12:
            try deserializer.decrease_container_depth()
            return .codeInvalid
        case 13:
            try deserializer.decrease_container_depth()
            return .creatingAccount
        case 14:
            try deserializer.decrease_container_depth()
            return .accountFailed
        case 15:
            try deserializer.decrease_container_depth()
            return .resettingPassword
        case 16:
            try deserializer.decrease_container_depth()
            return .resetFailed
        case 17:
            try deserializer.decrease_container_depth()
            return .syncing
        case 18:
            try deserializer.decrease_container_depth()
            return .syncDiverged
        case 19:
            try deserializer.decrease_container_depth()
            return .syncFailed
        case 20:
            try deserializer.decrease_container_depth()
            return .syncOffline
        case 21:
            try deserializer.decrease_container_depth()
            return .signInToSync
        case 22:
            try deserializer.decrease_container_depth()
            return .treeUnavailable
        case 23:
            try deserializer.decrease_container_depth()
            return .rateLimited
        case 24:
            try deserializer.decrease_container_depth()
            return .invalidEmail
        case 25:
            try deserializer.decrease_container_depth()
            return .passwordTooShort
        case 26:
            try deserializer.decrease_container_depth()
            return .passwordTooLong
        case 27:
            try deserializer.decrease_container_depth()
            return .emailTaken
        case 28:
            try deserializer.decrease_container_depth()
            return .purchaseConfirming
        case 29:
            try deserializer.decrease_container_depth()
            return .purchaseConfirmed
        case 30:
            try deserializer.decrease_container_depth()
            return .purchaseFailed
        case 31:
            try deserializer.decrease_container_depth()
            return .purchaseOwnedByOtherAccount
        case 32:
            try deserializer.decrease_container_depth()
            return .purchaseAccountMismatch
        case 33:
            try deserializer.decrease_container_depth()
            return .purchaseRevoked
        case 34:
            try deserializer.decrease_container_depth()
            return .purchaseNeedsAccount
        case 35:
            try deserializer.decrease_container_depth()
            return .trackRevoked
        case 36:
            try deserializer.decrease_container_depth()
            return .trackDownloadFailed
        case 37:
            try deserializer.decrease_container_depth()
            return .socialSignInFailed
        case 38:
            try deserializer.decrease_container_depth()
            return .socialEmailUnverified
        case 39:
            try deserializer.decrease_container_depth()
            return .socialProviderDisabled
        case 40:
            try deserializer.decrease_container_depth()
            return .providerReauthRequired
        case 41:
            try deserializer.decrease_container_depth()
            return .purchasePending
        case 42:
            try deserializer.decrease_container_depth()
            return .storeUnavailable
        case 43:
            try deserializer.decrease_container_depth()
            return .loginLocked
        case 44:
            try deserializer.decrease_container_depth()
            return .codeLocked
        default: throw DeserializationError.invalidInput(issue: "Unknown variant index for StatusKey: \(index)")
        }
    }

    public static func bincodeDeserialize(input: [UInt8]) throws -> StatusKey {
        let deserializer = BincodeDeserializer.init(input: input);
        let obj = try deserialize(deserializer: deserializer)
        if deserializer.get_buffer_offset() < input.count {
            throw DeserializationError.invalidInput(issue: "Some input bytes were not read")
        }
        return obj
    }
}

/// Pede ao shell que ofereça a avaliação na loja (App Store / Play Store).
/// Não tem retorno: a loja não diz se mostrou, e o Core não espera. Fire-and-forget.
indirect public enum StoreReviewOperation: Hashable, Equatable {
    case requestReview

    public func serialize<S: Serializer>(serializer: S) throws {
        try serializer.increase_container_depth()
        switch self {
        case .requestReview:
            try serializer.serialize_variant_index(value: 0)
        }
        try serializer.decrease_container_depth()
    }

    public func bincodeSerialize() throws -> [UInt8] {
        let serializer = BincodeSerializer.init();
        try self.serialize(serializer: serializer)
        return serializer.get_bytes()
    }

    public static func deserialize<D: Deserializer>(deserializer: D) throws -> StoreReviewOperation {
        let index = try deserializer.deserialize_variant_index()
        try deserializer.increase_container_depth()
        switch index {
        case 0:
            try deserializer.decrease_container_depth()
            return .requestReview
        default: throw DeserializationError.invalidInput(issue: "Unknown variant index for StoreReviewOperation: \(index)")
        }
    }

    public static func bincodeDeserialize(input: [UInt8]) throws -> StoreReviewOperation {
        let deserializer = BincodeDeserializer.init(input: input);
        let obj = try deserialize(deserializer: deserializer)
        if deserializer.get_buffer_offset() < input.count {
            throw DeserializationError.invalidInput(issue: "Some input bytes were not read")
        }
        return obj
    }
}

indirect public enum TelemetryOperation: Hashable, Equatable {
    case identify(userId: String)
    case track(event: String, properties: [String: String])
    /// Esquece a identidade: o que for enviado depois sai com um identificador anônimo
    /// novo. Na exclusão da conta, no logout e ao desligar a análise de uso.
    case reset
    /// O interruptor "Análise de uso". Desligado, o shell para a captura automática do
    /// SDK (abertura e fechamento do app); erros e medições seguem.
    case setAnalyticsEnabled(enabled: Bool)

    public func serialize<S: Serializer>(serializer: S) throws {
        try serializer.increase_container_depth()
        switch self {
        case .identify(let userId):
            try serializer.serialize_variant_index(value: 0)
            try serializer.serialize_str(value: userId)
        case .track(let event, let properties):
            try serializer.serialize_variant_index(value: 1)
            try serializer.serialize_str(value: event)
            try serializeMap(value: properties, serializer: serializer) { key, value, serializer in
                try serializer.serialize_str(value: key)
                try serializer.serialize_str(value: value)
            }
        case .reset:
            try serializer.serialize_variant_index(value: 2)
        case .setAnalyticsEnabled(let enabled):
            try serializer.serialize_variant_index(value: 3)
            try serializer.serialize_bool(value: enabled)
        }
        try serializer.decrease_container_depth()
    }

    public func bincodeSerialize() throws -> [UInt8] {
        let serializer = BincodeSerializer.init();
        try self.serialize(serializer: serializer)
        return serializer.get_bytes()
    }

    public static func deserialize<D: Deserializer>(deserializer: D) throws -> TelemetryOperation {
        let index = try deserializer.deserialize_variant_index()
        try deserializer.increase_container_depth()
        switch index {
        case 0:
            let userId = try deserializer.deserialize_str()
            try deserializer.decrease_container_depth()
            return .identify(userId: userId)
        case 1:
            let event = try deserializer.deserialize_str()
            let properties = try deserializeMap(deserializer: deserializer) { deserializer in
                let key = try deserializer.deserialize_str()
                let value = try deserializer.deserialize_str()
                return (key, value)
            }
            try deserializer.decrease_container_depth()
            return .track(event: event, properties: properties)
        case 2:
            try deserializer.decrease_container_depth()
            return .reset
        case 3:
            let enabled = try deserializer.deserialize_bool()
            try deserializer.decrease_container_depth()
            return .setAnalyticsEnabled(enabled: enabled)
        default: throw DeserializationError.invalidInput(issue: "Unknown variant index for TelemetryOperation: \(index)")
        }
    }

    public static func bincodeDeserialize(input: [UInt8]) throws -> TelemetryOperation {
        let deserializer = BincodeDeserializer.init(input: input);
        let obj = try deserialize(deserializer: deserializer)
        if deserializer.get_buffer_offset() < input.count {
            throw DeserializationError.invalidInput(issue: "Some input bytes were not read")
        }
        return obj
    }
}

/// Uma linha do "o que mudou" da tela de novo aceite. `summary` é conteúdo: vem do
/// servidor já na língua do app, e o cliente mostra como chegou.
public struct TermsChange: Hashable, Equatable {
    /// `terms` ou `privacy`.
    public var kind: String
    /// A versão em que a mudança entrou.
    public var version: UInt32
    public var change: TermsChangeKind
    /// O id da seção no documento, para abrir o documento com ela destacada.
    public var section: String
    public var summary: String

    public init(kind: String, version: UInt32, change: TermsChangeKind, section: String, summary: String) {
        self.kind = kind
        self.version = version
        self.change = change
        self.section = section
        self.summary = summary
    }

    public func serialize<S: Serializer>(serializer: S) throws {
        try serializer.increase_container_depth()
        try serializer.serialize_str(value: self.kind)
        try serializer.serialize_u32(value: self.version)
        try self.change.serialize(serializer: serializer)
        try serializer.serialize_str(value: self.section)
        try serializer.serialize_str(value: self.summary)
        try serializer.decrease_container_depth()
    }

    public func bincodeSerialize() throws -> [UInt8] {
        let serializer = BincodeSerializer.init();
        try self.serialize(serializer: serializer)
        return serializer.get_bytes()
    }

    public static func deserialize<D: Deserializer>(deserializer: D) throws -> TermsChange {
        try deserializer.increase_container_depth()
        let kind = try deserializer.deserialize_str()
        let version = try deserializer.deserialize_u32()
        let change = try LogN.TermsChangeKind.deserialize(deserializer: deserializer)
        let section = try deserializer.deserialize_str()
        let summary = try deserializer.deserialize_str()
        try deserializer.decrease_container_depth()
        return TermsChange(kind: kind, version: version, change: change, section: section, summary: summary)
    }

    public static func bincodeDeserialize(input: [UInt8]) throws -> TermsChange {
        let deserializer = BincodeDeserializer.init(input: input);
        let obj = try deserialize(deserializer: deserializer)
        if deserializer.get_buffer_offset() < input.count {
            throw DeserializationError.invalidInput(issue: "Some input bytes were not read")
        }
        return obj
    }
}

/// O sinal de uma mudança nos termos, como o diff do design: `+`, `~`, `−`.
indirect public enum TermsChangeKind: Hashable, Equatable {
    case added
    case changed
    case removed

    public func serialize<S: Serializer>(serializer: S) throws {
        try serializer.increase_container_depth()
        switch self {
        case .added:
            try serializer.serialize_variant_index(value: 0)
        case .changed:
            try serializer.serialize_variant_index(value: 1)
        case .removed:
            try serializer.serialize_variant_index(value: 2)
        }
        try serializer.decrease_container_depth()
    }

    public func bincodeSerialize() throws -> [UInt8] {
        let serializer = BincodeSerializer.init();
        try self.serialize(serializer: serializer)
        return serializer.get_bytes()
    }

    public static func deserialize<D: Deserializer>(deserializer: D) throws -> TermsChangeKind {
        let index = try deserializer.deserialize_variant_index()
        try deserializer.increase_container_depth()
        switch index {
        case 0:
            try deserializer.decrease_container_depth()
            return .added
        case 1:
            try deserializer.decrease_container_depth()
            return .changed
        case 2:
            try deserializer.decrease_container_depth()
            return .removed
        default: throw DeserializationError.invalidInput(issue: "Unknown variant index for TermsChangeKind: \(index)")
        }
    }

    public static func bincodeDeserialize(input: [UInt8]) throws -> TermsChangeKind {
        let deserializer = BincodeDeserializer.init(input: input);
        let obj = try deserialize(deserializer: deserializer)
        if deserializer.get_buffer_offset() < input.count {
            throw DeserializationError.invalidInput(issue: "Some input bytes were not read")
        }
        return obj
    }
}

/// A tela que cobre o app quando há versão relevante para aceitar (ADR 0020).
public struct TermsUpdateViewModel: Hashable, Equatable {
    /// A última versão aceita pela conta, e a data de vigência dela (AAAA-MM-DD). Zero
    /// e vazia quando a conta nunca aceitou.
    public var fromVersion: UInt32
    public var fromDate: String
    /// A vigente, e a data de vigência dela.
    public var toVersion: UInt32
    public var toDate: String
    /// Quantas versões ficaram entre a aceita e a vigente, contando a vigente.
    public var versionsSkipped: UInt32
    /// Todas as mudanças das versões puladas, na ordem.
    public var changes: [TermsChange]
    /// As seções a destacar ao abrir cada documento.
    public var termsSections: [String]
    public var privacySections: [String]
    /// O aceite foi enviado e espera o servidor.
    public var accepting: Bool

    public init(fromVersion: UInt32, fromDate: String, toVersion: UInt32, toDate: String, versionsSkipped: UInt32, changes: [TermsChange], termsSections: [String], privacySections: [String], accepting: Bool) {
        self.fromVersion = fromVersion
        self.fromDate = fromDate
        self.toVersion = toVersion
        self.toDate = toDate
        self.versionsSkipped = versionsSkipped
        self.changes = changes
        self.termsSections = termsSections
        self.privacySections = privacySections
        self.accepting = accepting
    }

    public func serialize<S: Serializer>(serializer: S) throws {
        try serializer.increase_container_depth()
        try serializer.serialize_u32(value: self.fromVersion)
        try serializer.serialize_str(value: self.fromDate)
        try serializer.serialize_u32(value: self.toVersion)
        try serializer.serialize_str(value: self.toDate)
        try serializer.serialize_u32(value: self.versionsSkipped)
        try serializeArray(value: self.changes, serializer: serializer) { item, serializer in
            try item.serialize(serializer: serializer)
        }
        try serializeArray(value: self.termsSections, serializer: serializer) { item, serializer in
            try serializer.serialize_str(value: item)
        }
        try serializeArray(value: self.privacySections, serializer: serializer) { item, serializer in
            try serializer.serialize_str(value: item)
        }
        try serializer.serialize_bool(value: self.accepting)
        try serializer.decrease_container_depth()
    }

    public func bincodeSerialize() throws -> [UInt8] {
        let serializer = BincodeSerializer.init();
        try self.serialize(serializer: serializer)
        return serializer.get_bytes()
    }

    public static func deserialize<D: Deserializer>(deserializer: D) throws -> TermsUpdateViewModel {
        try deserializer.increase_container_depth()
        let fromVersion = try deserializer.deserialize_u32()
        let fromDate = try deserializer.deserialize_str()
        let toVersion = try deserializer.deserialize_u32()
        let toDate = try deserializer.deserialize_str()
        let versionsSkipped = try deserializer.deserialize_u32()
        let changes = try deserializeArray(deserializer: deserializer) { deserializer in
            try LogN.TermsChange.deserialize(deserializer: deserializer)
        }
        let termsSections = try deserializeArray(deserializer: deserializer) { deserializer in
            try deserializer.deserialize_str()
        }
        let privacySections = try deserializeArray(deserializer: deserializer) { deserializer in
            try deserializer.deserialize_str()
        }
        let accepting = try deserializer.deserialize_bool()
        try deserializer.decrease_container_depth()
        return TermsUpdateViewModel(fromVersion: fromVersion, fromDate: fromDate, toVersion: toVersion, toDate: toDate, versionsSkipped: versionsSkipped, changes: changes, termsSections: termsSections, privacySections: privacySections, accepting: accepting)
    }

    public static func bincodeDeserialize(input: [UInt8]) throws -> TermsUpdateViewModel {
        let deserializer = BincodeDeserializer.init(input: input);
        let obj = try deserialize(deserializer: deserializer)
        if deserializer.get_buffer_offset() < input.count {
            throw DeserializationError.invalidInput(issue: "Some input bytes were not read")
        }
        return obj
    }
}

indirect public enum TimeRequest: Hashable, Equatable {
    case now
    case notifyAt(id: TimerId, instant: Instant)
    case notifyAfter(id: TimerId, duration: Duration)
    case clear(id: TimerId)

    public func serialize<S: Serializer>(serializer: S) throws {
        try serializer.increase_container_depth()
        switch self {
        case .now:
            try serializer.serialize_variant_index(value: 0)
        case .notifyAt(let id, let instant):
            try serializer.serialize_variant_index(value: 1)
            try id.serialize(serializer: serializer)
            try instant.serialize(serializer: serializer)
        case .notifyAfter(let id, let duration):
            try serializer.serialize_variant_index(value: 2)
            try id.serialize(serializer: serializer)
            try duration.serialize(serializer: serializer)
        case .clear(let id):
            try serializer.serialize_variant_index(value: 3)
            try id.serialize(serializer: serializer)
        }
        try serializer.decrease_container_depth()
    }

    public func bincodeSerialize() throws -> [UInt8] {
        let serializer = BincodeSerializer.init();
        try self.serialize(serializer: serializer)
        return serializer.get_bytes()
    }

    public static func deserialize<D: Deserializer>(deserializer: D) throws -> TimeRequest {
        let index = try deserializer.deserialize_variant_index()
        try deserializer.increase_container_depth()
        switch index {
        case 0:
            try deserializer.decrease_container_depth()
            return .now
        case 1:
            let id = try LogN.TimerId.deserialize(deserializer: deserializer)
            let instant = try LogN.Instant.deserialize(deserializer: deserializer)
            try deserializer.decrease_container_depth()
            return .notifyAt(id: id, instant: instant)
        case 2:
            let id = try LogN.TimerId.deserialize(deserializer: deserializer)
            let duration = try LogN.Duration.deserialize(deserializer: deserializer)
            try deserializer.decrease_container_depth()
            return .notifyAfter(id: id, duration: duration)
        case 3:
            let id = try LogN.TimerId.deserialize(deserializer: deserializer)
            try deserializer.decrease_container_depth()
            return .clear(id: id)
        default: throw DeserializationError.invalidInput(issue: "Unknown variant index for TimeRequest: \(index)")
        }
    }

    public static func bincodeDeserialize(input: [UInt8]) throws -> TimeRequest {
        let deserializer = BincodeDeserializer.init(input: input);
        let obj = try deserialize(deserializer: deserializer)
        if deserializer.get_buffer_offset() < input.count {
            throw DeserializationError.invalidInput(issue: "Some input bytes were not read")
        }
        return obj
    }
}

public struct TimerId: Hashable, Equatable {
    public var value: UInt64

    public init(value: UInt64) {
        self.value = value
    }

    public func serialize<S: Serializer>(serializer: S) throws {
        try serializer.increase_container_depth()
        try serializer.serialize_u64(value: self.value)
        try serializer.decrease_container_depth()
    }

    public func bincodeSerialize() throws -> [UInt8] {
        let serializer = BincodeSerializer.init();
        try self.serialize(serializer: serializer)
        return serializer.get_bytes()
    }

    public static func deserialize<D: Deserializer>(deserializer: D) throws -> TimerId {
        try deserializer.increase_container_depth()
        let value = try deserializer.deserialize_u64()
        try deserializer.decrease_container_depth()
        return TimerId(value: value)
    }

    public static func bincodeDeserialize(input: [UInt8]) throws -> TimerId {
        let deserializer = BincodeDeserializer.init(input: input);
        let obj = try deserialize(deserializer: deserializer)
        if deserializer.get_buffer_offset() < input.count {
            throw DeserializationError.invalidInput(issue: "Some input bytes were not read")
        }
        return obj
    }
}

/// Um nó na lista do detalhe da trilha.
public struct TrackNodeRow: Hashable, Equatable {
    public var id: String
    public var name: String
    /// Abre sem compra: a amostra.
    public var free: Bool
    public var done: Bool
    /// O próximo a jogar.
    public var active: Bool

    public init(id: String, name: String, free: Bool, done: Bool, active: Bool) {
        self.id = id
        self.name = name
        self.free = free
        self.done = done
        self.active = active
    }

    public func serialize<S: Serializer>(serializer: S) throws {
        try serializer.increase_container_depth()
        try serializer.serialize_str(value: self.id)
        try serializer.serialize_str(value: self.name)
        try serializer.serialize_bool(value: self.free)
        try serializer.serialize_bool(value: self.done)
        try serializer.serialize_bool(value: self.active)
        try serializer.decrease_container_depth()
    }

    public func bincodeSerialize() throws -> [UInt8] {
        let serializer = BincodeSerializer.init();
        try self.serialize(serializer: serializer)
        return serializer.get_bytes()
    }

    public static func deserialize<D: Deserializer>(deserializer: D) throws -> TrackNodeRow {
        try deserializer.increase_container_depth()
        let id = try deserializer.deserialize_str()
        let name = try deserializer.deserialize_str()
        let free = try deserializer.deserialize_bool()
        let done = try deserializer.deserialize_bool()
        let active = try deserializer.deserialize_bool()
        try deserializer.decrease_container_depth()
        return TrackNodeRow(id: id, name: name, free: free, done: done, active: active)
    }

    public static func bincodeDeserialize(input: [UInt8]) throws -> TrackNodeRow {
        let deserializer = BincodeDeserializer.init(input: input);
        let obj = try deserialize(deserializer: deserializer)
        if deserializer.get_buffer_offset() < input.count {
            throw DeserializationError.invalidInput(issue: "Some input bytes were not read")
        }
        return obj
    }
}

/// A trilha como a tela a vê: catálogo, detalhe, compra e o que está no aparelho.
public struct TrackView: Hashable, Equatable {
    public var id: String
    /// A trilha gratuita, a principal.
    public var isFree: Bool
    public var name: String
    public var description: String
    public var author: String
    /// A cor do balão da trilha, `#RRGGBB`.
    public var color: String
    /// O produto da App Store. É daqui que o app compra, nunca de um id montado.
    public var productId: String
    public var nodeCount: UInt32
    public var problemCount: UInt32
    /// Línguas publicadas, na forma do banco (`pt-BR`, `en`, `es`).
    public var languages: [String]
    /// É a trilha que a árvore mostra agora.
    public var selected: Bool
    public var owned: Bool
    /// A compra foi revogada; `revoked_reason` diz por quê.
    public var revoked: Bool
    public var revokedReason: String
    public var discontinued: Bool
    /// Nós conquistados nesta trilha.
    public var nodesDone: UInt32
    /// Nós que só abrem com compra.
    public var closedNodeCount: UInt32
    /// Balões subidos na amostra, para a oferta do fim dela.
    public var sampleBalloons: UInt32
    /// Todos os problemas da amostra já renderam XP.
    public var sampleDone: Bool
    /// XP ganho nesta trilha.
    public var trackXp: Int32
    /// O conteúdo fechado está no aparelho e abre.
    public var downloaded: Bool
    /// Tamanho do pacote no aparelho, em bytes. Zero quando não está baixado.
    public var downloadBytes: UInt64
    public var offline: OfflineState
    /// Dias inteiros até a licença offline vencer. Zero sem licença ou vencida.
    public var offlineDaysLeft: UInt32
    /// Dias desde o último contato com o servidor, pela emissão da licença.
    public var daysSinceContact: UInt32
    /// Até quando a licença vale, em segundos desde a época. Zero sem licença.
    public var validUntil: Int64
    /// Os nós da trilha, na ordem da árvore, para o detalhe (1d).
    public var nodes: [TrackNodeRow]

    public init(id: String, isFree: Bool, name: String, description: String, author: String, color: String, productId: String, nodeCount: UInt32, problemCount: UInt32, languages: [String], selected: Bool, owned: Bool, revoked: Bool, revokedReason: String, discontinued: Bool, nodesDone: UInt32, closedNodeCount: UInt32, sampleBalloons: UInt32, sampleDone: Bool, trackXp: Int32, downloaded: Bool, downloadBytes: UInt64, offline: OfflineState, offlineDaysLeft: UInt32, daysSinceContact: UInt32, validUntil: Int64, nodes: [TrackNodeRow]) {
        self.id = id
        self.isFree = isFree
        self.name = name
        self.description = description
        self.author = author
        self.color = color
        self.productId = productId
        self.nodeCount = nodeCount
        self.problemCount = problemCount
        self.languages = languages
        self.selected = selected
        self.owned = owned
        self.revoked = revoked
        self.revokedReason = revokedReason
        self.discontinued = discontinued
        self.nodesDone = nodesDone
        self.closedNodeCount = closedNodeCount
        self.sampleBalloons = sampleBalloons
        self.sampleDone = sampleDone
        self.trackXp = trackXp
        self.downloaded = downloaded
        self.downloadBytes = downloadBytes
        self.offline = offline
        self.offlineDaysLeft = offlineDaysLeft
        self.daysSinceContact = daysSinceContact
        self.validUntil = validUntil
        self.nodes = nodes
    }

    public func serialize<S: Serializer>(serializer: S) throws {
        try serializer.increase_container_depth()
        try serializer.serialize_str(value: self.id)
        try serializer.serialize_bool(value: self.isFree)
        try serializer.serialize_str(value: self.name)
        try serializer.serialize_str(value: self.description)
        try serializer.serialize_str(value: self.author)
        try serializer.serialize_str(value: self.color)
        try serializer.serialize_str(value: self.productId)
        try serializer.serialize_u32(value: self.nodeCount)
        try serializer.serialize_u32(value: self.problemCount)
        try serializeArray(value: self.languages, serializer: serializer) { item, serializer in
            try serializer.serialize_str(value: item)
        }
        try serializer.serialize_bool(value: self.selected)
        try serializer.serialize_bool(value: self.owned)
        try serializer.serialize_bool(value: self.revoked)
        try serializer.serialize_str(value: self.revokedReason)
        try serializer.serialize_bool(value: self.discontinued)
        try serializer.serialize_u32(value: self.nodesDone)
        try serializer.serialize_u32(value: self.closedNodeCount)
        try serializer.serialize_u32(value: self.sampleBalloons)
        try serializer.serialize_bool(value: self.sampleDone)
        try serializer.serialize_i32(value: self.trackXp)
        try serializer.serialize_bool(value: self.downloaded)
        try serializer.serialize_u64(value: self.downloadBytes)
        try self.offline.serialize(serializer: serializer)
        try serializer.serialize_u32(value: self.offlineDaysLeft)
        try serializer.serialize_u32(value: self.daysSinceContact)
        try serializer.serialize_i64(value: self.validUntil)
        try serializeArray(value: self.nodes, serializer: serializer) { item, serializer in
            try item.serialize(serializer: serializer)
        }
        try serializer.decrease_container_depth()
    }

    public func bincodeSerialize() throws -> [UInt8] {
        let serializer = BincodeSerializer.init();
        try self.serialize(serializer: serializer)
        return serializer.get_bytes()
    }

    public static func deserialize<D: Deserializer>(deserializer: D) throws -> TrackView {
        try deserializer.increase_container_depth()
        let id = try deserializer.deserialize_str()
        let isFree = try deserializer.deserialize_bool()
        let name = try deserializer.deserialize_str()
        let description = try deserializer.deserialize_str()
        let author = try deserializer.deserialize_str()
        let color = try deserializer.deserialize_str()
        let productId = try deserializer.deserialize_str()
        let nodeCount = try deserializer.deserialize_u32()
        let problemCount = try deserializer.deserialize_u32()
        let languages = try deserializeArray(deserializer: deserializer) { deserializer in
            try deserializer.deserialize_str()
        }
        let selected = try deserializer.deserialize_bool()
        let owned = try deserializer.deserialize_bool()
        let revoked = try deserializer.deserialize_bool()
        let revokedReason = try deserializer.deserialize_str()
        let discontinued = try deserializer.deserialize_bool()
        let nodesDone = try deserializer.deserialize_u32()
        let closedNodeCount = try deserializer.deserialize_u32()
        let sampleBalloons = try deserializer.deserialize_u32()
        let sampleDone = try deserializer.deserialize_bool()
        let trackXp = try deserializer.deserialize_i32()
        let downloaded = try deserializer.deserialize_bool()
        let downloadBytes = try deserializer.deserialize_u64()
        let offline = try LogN.OfflineState.deserialize(deserializer: deserializer)
        let offlineDaysLeft = try deserializer.deserialize_u32()
        let daysSinceContact = try deserializer.deserialize_u32()
        let validUntil = try deserializer.deserialize_i64()
        let nodes = try deserializeArray(deserializer: deserializer) { deserializer in
            try LogN.TrackNodeRow.deserialize(deserializer: deserializer)
        }
        try deserializer.decrease_container_depth()
        return TrackView(id: id, isFree: isFree, name: name, description: description, author: author, color: color, productId: productId, nodeCount: nodeCount, problemCount: problemCount, languages: languages, selected: selected, owned: owned, revoked: revoked, revokedReason: revokedReason, discontinued: discontinued, nodesDone: nodesDone, closedNodeCount: closedNodeCount, sampleBalloons: sampleBalloons, sampleDone: sampleDone, trackXp: trackXp, downloaded: downloaded, downloadBytes: downloadBytes, offline: offline, offlineDaysLeft: offlineDaysLeft, daysSinceContact: daysSinceContact, validUntil: validUntil, nodes: nodes)
    }

    public static func bincodeDeserialize(input: [UInt8]) throws -> TrackView {
        let deserializer = BincodeDeserializer.init(input: input);
        let obj = try deserialize(deserializer: deserializer)
        if deserializer.get_buffer_offset() < input.count {
            throw DeserializationError.invalidInput(issue: "Some input bytes were not read")
        }
        return obj
    }
}

/// Por que o cartão de armadilha abriu. O texto do cartão sai do catálogo do cliente:
/// o Core escrevia "TRAP CLÁSSICA" e "O relógio da questão zerou" em português.
indirect public enum TrapKind: Hashable, Equatable {
    case wrongAnswer
    case timeLimit

    public func serialize<S: Serializer>(serializer: S) throws {
        try serializer.increase_container_depth()
        switch self {
        case .wrongAnswer:
            try serializer.serialize_variant_index(value: 0)
        case .timeLimit:
            try serializer.serialize_variant_index(value: 1)
        }
        try serializer.decrease_container_depth()
    }

    public func bincodeSerialize() throws -> [UInt8] {
        let serializer = BincodeSerializer.init();
        try self.serialize(serializer: serializer)
        return serializer.get_bytes()
    }

    public static func deserialize<D: Deserializer>(deserializer: D) throws -> TrapKind {
        let index = try deserializer.deserialize_variant_index()
        try deserializer.increase_container_depth()
        switch index {
        case 0:
            try deserializer.decrease_container_depth()
            return .wrongAnswer
        case 1:
            try deserializer.decrease_container_depth()
            return .timeLimit
        default: throw DeserializationError.invalidInput(issue: "Unknown variant index for TrapKind: \(index)")
        }
    }

    public static func bincodeDeserialize(input: [UInt8]) throws -> TrapKind {
        let deserializer = BincodeDeserializer.init(input: input);
        let obj = try deserialize(deserializer: deserializer)
        if deserializer.get_buffer_offset() < input.count {
            throw DeserializationError.invalidInput(issue: "Some input bytes were not read")
        }
        return obj
    }
}

/// The value stored under a key.
/// 
/// `Value::None` is used to represent the absence of a value.
/// 
/// Note: we can't use `Option` here because generics are not currently
/// supported across the FFI boundary, when using the builtin typegen.
indirect public enum Value: Hashable, Equatable {
    case none
    case bytes([UInt8])

    public func serialize<S: Serializer>(serializer: S) throws {
        try serializer.increase_container_depth()
        switch self {
        case .none:
            try serializer.serialize_variant_index(value: 0)
        case .bytes(let x):
            try serializer.serialize_variant_index(value: 1)
            try serializeArray(value: x, serializer: serializer) { item, serializer in
                try serializer.serialize_u8(value: item)
            }
        }
        try serializer.decrease_container_depth()
    }

    public func bincodeSerialize() throws -> [UInt8] {
        let serializer = BincodeSerializer.init();
        try self.serialize(serializer: serializer)
        return serializer.get_bytes()
    }

    public static func deserialize<D: Deserializer>(deserializer: D) throws -> Value {
        let index = try deserializer.deserialize_variant_index()
        try deserializer.increase_container_depth()
        switch index {
        case 0:
            try deserializer.decrease_container_depth()
            return .none
        case 1:
            let x = try deserializeArray(deserializer: deserializer) { deserializer in
                try deserializer.deserialize_u8()
            }
            try deserializer.decrease_container_depth()
            return .bytes(x)
        default: throw DeserializationError.invalidInput(issue: "Unknown variant index for Value: \(index)")
        }
    }

    public static func bincodeDeserialize(input: [UInt8]) throws -> Value {
        let deserializer = BincodeDeserializer.init(input: input);
        let obj = try deserialize(deserializer: deserializer)
        if deserializer.get_buffer_offset() < input.count {
            throw DeserializationError.invalidInput(issue: "Some input bytes were not read")
        }
        return obj
    }
}

public struct ViewModel: Hashable, Equatable {
    /// O que dizer ao jogador, como chave — o cliente traduz. Nunca frase pronta:
    /// o Core não sabe em que idioma o app está.
    public var status: StatusKey
    public var pendingSyncCount: UInt32
    public var isSyncing: Bool
    public var isFetching: Bool
    public var isAuthenticating: Bool
    /// Tem credencial para falar com o servidor agora.
    public var hasAccessToken: Bool
    /// Tem sessão — com ou sem rede. É isto que decide se o app abre no jogo ou no login.
    public var hasSession: Bool
    /// A sessão está em pé sem ter falado com o servidor nesta abertura.
    public var isOfflineSession: Bool
    /// A trilha na tela é a que viajou no bundle, congelada quando o build saiu.
    /// O cliente avisa: pode haver desafio que este app ainda não conhece.
    public var trailFromBundle: Bool
    /// Quando essa semente foi gerada, em ISO 8601. O cliente formata na língua dele.
    public var trailGeneratedAt: String
    /// A última partida acabou porque o jogador saiu, não porque ela terminou. A tela
    /// volta direto para a trilha: quem desistiu não tem relatório para ler.
    public var matchLeft: Bool
    public var isGuest: Bool
    public var locale: String
    public var challenges: [Challenge]
    public var nodes: [SkillNode]
    public var otpEmail: String
    /// E-mail da conta em sessão. Vazio no visitante.
    public var accountEmail: String
    public var otpVerified: Bool
    public var globalXp: Int32
    public var bugsFound: Int32
    public var dryRunsCompleted: Int32
    /// Progressão. Calculada aqui, nunca no cliente.
    public var level: Int32
    public var xpIntoLevel: Int32
    public var xpForLevel: Int32
    public var xpToNextLevel: Int32
    public var challengesCompleted: Int32
    public var balloonsUp: Int32
    /// Acabou de sair e ainda dá para desfazer.
    public var justLoggedOut: Bool
    /// Acabou de trocar a senha: a tela de redefinição pode fechar.
    public var passwordResetDone: Bool
    /// Primeiro nome derivado do e-mail, para a despedida.
    public var displayName: String
    public var matchView: MatchViewModel
    public var contestName: String
    public var standingsGlobal: [StandingRow]
    public var standingsHome: [StandingRow]
    public var userStanding: StandingRow
    public var scoreboard: [ScoreboardRow]
    /// Ranking e telão são dados de exemplo, não de jogadores de verdade.
    /// 
    /// Sem este aviso o jogador lia "você está em 42º" como fato. Quem sabe de onde os
    /// dados vêm é o Core, então é ele quem desliga o aviso quando o placar tiver API.
    public var standingsAreSample: Bool
    /// Segundos até login, verificação, cadastro e troca de senha voltarem a valer.
    /// Zero quando não há bloqueio.
    public var authCooldownSeconds: UInt32
    /// Segundos até enviar ou reenviar código voltar a valer. Inclui o bloqueio geral:
    /// o limite por IP também barra o envio.
    public var resendCooldownSeconds: UInt32
    /// Já se sabe quais versões dos termos e da política o cadastro aceita. Sem isso o
    /// cadastro não tem o que mandar, e a tela segura o envio do código.
    public var legalVersionsReady: Bool
    /// Idade mínima para criar conta no país do aparelho — o N de "tenho N anos ou
    /// mais". Enquanto o servidor não responde, a idade padrão.
    public var minAge: UInt32
    /// Até quando a conta que acabou de pedir exclusão pode ser recuperada, em segundos
    /// desde a época. 0 = sem aviso. O cliente formata a data.
    public var deletionPurgeAfter: Int64
    /// Mostrar, uma vez, que a exclusão foi cancelada e a conta voltou.
    public var accountRestoredNotice: Bool
    /// O login pelo provedor ainda não tem conta: o shell mostra idade e termos e
    /// responde com `CompleteSocialSignup` ou `CancelSocialSignup`.
    public var socialSignupRequired: Bool
    /// Estado do interruptor "Análise de uso".
    public var analyticsEnabled: Bool
    /// A splash da abertura.
    public var boot: BootViewModel
    /// Versão relevante dos termos para aceitar: a tela cobre o app até aceitar ou sair
    /// (ADR 0020).
    public var termsUpdate: TermsUpdateViewModel?
    /// Só mudanças não relevantes: a faixa discreta na árvore, até fechar.
    public var termsNotice: Bool
    /// E-mail para o login já vir preenchido depois de uma sessão que acabou. Vazio
    /// quando não há.
    public var resumeEmail: String
    /// Id da conta em sessão. O shell põe no `appAccountToken` da compra, e o servidor
    /// confere que quem manda a transação é quem comprou. Vazio no visitante.
    public var accountUserId: String
    /// O catálogo: a principal primeiro, depois as pagas, com compra e o que está baixado.
    public var tracks: [TrackView]
    /// A trilha que a árvore mostra. `nodes` já vem filtrado por ela.
    public var currentTrack: TrackView
    /// O selo do botão Trilhas.
    public var catalogBadge: CatalogBadge
    /// Mostrar "Por onde começar?" (uma vez por aparelho).
    public var showOnboarding: Bool
    public var purchaseFlow: PurchaseFlowView
    public var restoreResult: RestoreResultView
    /// A oferta do fim da amostra, quando a partida que acabou foi a amostra.
    public var sampleOffer: SampleOfferView
    /// Transações confirmadas pelo servidor que o shell tem de finalizar na loja e
    /// devolver com `PurchaseFinished`.
    public var purchasesToFinish: [String]
    public var purchaseInFlight: Bool

    public init(status: StatusKey, pendingSyncCount: UInt32, isSyncing: Bool, isFetching: Bool, isAuthenticating: Bool, hasAccessToken: Bool, hasSession: Bool, isOfflineSession: Bool, trailFromBundle: Bool, trailGeneratedAt: String, matchLeft: Bool, isGuest: Bool, locale: String, challenges: [Challenge], nodes: [SkillNode], otpEmail: String, accountEmail: String, otpVerified: Bool, globalXp: Int32, bugsFound: Int32, dryRunsCompleted: Int32, level: Int32, xpIntoLevel: Int32, xpForLevel: Int32, xpToNextLevel: Int32, challengesCompleted: Int32, balloonsUp: Int32, justLoggedOut: Bool, passwordResetDone: Bool, displayName: String, matchView: MatchViewModel, contestName: String, standingsGlobal: [StandingRow], standingsHome: [StandingRow], userStanding: StandingRow, scoreboard: [ScoreboardRow], standingsAreSample: Bool, authCooldownSeconds: UInt32, resendCooldownSeconds: UInt32, legalVersionsReady: Bool, minAge: UInt32, deletionPurgeAfter: Int64, accountRestoredNotice: Bool, socialSignupRequired: Bool, analyticsEnabled: Bool, boot: BootViewModel, termsUpdate: TermsUpdateViewModel?, termsNotice: Bool, resumeEmail: String, accountUserId: String, tracks: [TrackView], currentTrack: TrackView, catalogBadge: CatalogBadge, showOnboarding: Bool, purchaseFlow: PurchaseFlowView, restoreResult: RestoreResultView, sampleOffer: SampleOfferView, purchasesToFinish: [String], purchaseInFlight: Bool) {
        self.status = status
        self.pendingSyncCount = pendingSyncCount
        self.isSyncing = isSyncing
        self.isFetching = isFetching
        self.isAuthenticating = isAuthenticating
        self.hasAccessToken = hasAccessToken
        self.hasSession = hasSession
        self.isOfflineSession = isOfflineSession
        self.trailFromBundle = trailFromBundle
        self.trailGeneratedAt = trailGeneratedAt
        self.matchLeft = matchLeft
        self.isGuest = isGuest
        self.locale = locale
        self.challenges = challenges
        self.nodes = nodes
        self.otpEmail = otpEmail
        self.accountEmail = accountEmail
        self.otpVerified = otpVerified
        self.globalXp = globalXp
        self.bugsFound = bugsFound
        self.dryRunsCompleted = dryRunsCompleted
        self.level = level
        self.xpIntoLevel = xpIntoLevel
        self.xpForLevel = xpForLevel
        self.xpToNextLevel = xpToNextLevel
        self.challengesCompleted = challengesCompleted
        self.balloonsUp = balloonsUp
        self.justLoggedOut = justLoggedOut
        self.passwordResetDone = passwordResetDone
        self.displayName = displayName
        self.matchView = matchView
        self.contestName = contestName
        self.standingsGlobal = standingsGlobal
        self.standingsHome = standingsHome
        self.userStanding = userStanding
        self.scoreboard = scoreboard
        self.standingsAreSample = standingsAreSample
        self.authCooldownSeconds = authCooldownSeconds
        self.resendCooldownSeconds = resendCooldownSeconds
        self.legalVersionsReady = legalVersionsReady
        self.minAge = minAge
        self.deletionPurgeAfter = deletionPurgeAfter
        self.accountRestoredNotice = accountRestoredNotice
        self.socialSignupRequired = socialSignupRequired
        self.analyticsEnabled = analyticsEnabled
        self.boot = boot
        self.termsUpdate = termsUpdate
        self.termsNotice = termsNotice
        self.resumeEmail = resumeEmail
        self.accountUserId = accountUserId
        self.tracks = tracks
        self.currentTrack = currentTrack
        self.catalogBadge = catalogBadge
        self.showOnboarding = showOnboarding
        self.purchaseFlow = purchaseFlow
        self.restoreResult = restoreResult
        self.sampleOffer = sampleOffer
        self.purchasesToFinish = purchasesToFinish
        self.purchaseInFlight = purchaseInFlight
    }

    public func serialize<S: Serializer>(serializer: S) throws {
        try serializer.increase_container_depth()
        try self.status.serialize(serializer: serializer)
        try serializer.serialize_u32(value: self.pendingSyncCount)
        try serializer.serialize_bool(value: self.isSyncing)
        try serializer.serialize_bool(value: self.isFetching)
        try serializer.serialize_bool(value: self.isAuthenticating)
        try serializer.serialize_bool(value: self.hasAccessToken)
        try serializer.serialize_bool(value: self.hasSession)
        try serializer.serialize_bool(value: self.isOfflineSession)
        try serializer.serialize_bool(value: self.trailFromBundle)
        try serializer.serialize_str(value: self.trailGeneratedAt)
        try serializer.serialize_bool(value: self.matchLeft)
        try serializer.serialize_bool(value: self.isGuest)
        try serializer.serialize_str(value: self.locale)
        try serializeArray(value: self.challenges, serializer: serializer) { item, serializer in
            try item.serialize(serializer: serializer)
        }
        try serializeArray(value: self.nodes, serializer: serializer) { item, serializer in
            try item.serialize(serializer: serializer)
        }
        try serializer.serialize_str(value: self.otpEmail)
        try serializer.serialize_str(value: self.accountEmail)
        try serializer.serialize_bool(value: self.otpVerified)
        try serializer.serialize_i32(value: self.globalXp)
        try serializer.serialize_i32(value: self.bugsFound)
        try serializer.serialize_i32(value: self.dryRunsCompleted)
        try serializer.serialize_i32(value: self.level)
        try serializer.serialize_i32(value: self.xpIntoLevel)
        try serializer.serialize_i32(value: self.xpForLevel)
        try serializer.serialize_i32(value: self.xpToNextLevel)
        try serializer.serialize_i32(value: self.challengesCompleted)
        try serializer.serialize_i32(value: self.balloonsUp)
        try serializer.serialize_bool(value: self.justLoggedOut)
        try serializer.serialize_bool(value: self.passwordResetDone)
        try serializer.serialize_str(value: self.displayName)
        try self.matchView.serialize(serializer: serializer)
        try serializer.serialize_str(value: self.contestName)
        try serializeArray(value: self.standingsGlobal, serializer: serializer) { item, serializer in
            try item.serialize(serializer: serializer)
        }
        try serializeArray(value: self.standingsHome, serializer: serializer) { item, serializer in
            try item.serialize(serializer: serializer)
        }
        try self.userStanding.serialize(serializer: serializer)
        try serializeArray(value: self.scoreboard, serializer: serializer) { item, serializer in
            try item.serialize(serializer: serializer)
        }
        try serializer.serialize_bool(value: self.standingsAreSample)
        try serializer.serialize_u32(value: self.authCooldownSeconds)
        try serializer.serialize_u32(value: self.resendCooldownSeconds)
        try serializer.serialize_bool(value: self.legalVersionsReady)
        try serializer.serialize_u32(value: self.minAge)
        try serializer.serialize_i64(value: self.deletionPurgeAfter)
        try serializer.serialize_bool(value: self.accountRestoredNotice)
        try serializer.serialize_bool(value: self.socialSignupRequired)
        try serializer.serialize_bool(value: self.analyticsEnabled)
        try self.boot.serialize(serializer: serializer)
        try serializeOption(value: self.termsUpdate, serializer: serializer) { value, serializer in
            try value.serialize(serializer: serializer)
        }
        try serializer.serialize_bool(value: self.termsNotice)
        try serializer.serialize_str(value: self.resumeEmail)
        try serializer.serialize_str(value: self.accountUserId)
        try serializeArray(value: self.tracks, serializer: serializer) { item, serializer in
            try item.serialize(serializer: serializer)
        }
        try self.currentTrack.serialize(serializer: serializer)
        try self.catalogBadge.serialize(serializer: serializer)
        try serializer.serialize_bool(value: self.showOnboarding)
        try self.purchaseFlow.serialize(serializer: serializer)
        try self.restoreResult.serialize(serializer: serializer)
        try self.sampleOffer.serialize(serializer: serializer)
        try serializeArray(value: self.purchasesToFinish, serializer: serializer) { item, serializer in
            try serializer.serialize_str(value: item)
        }
        try serializer.serialize_bool(value: self.purchaseInFlight)
        try serializer.decrease_container_depth()
    }

    public func bincodeSerialize() throws -> [UInt8] {
        let serializer = BincodeSerializer.init();
        try self.serialize(serializer: serializer)
        return serializer.get_bytes()
    }

    public static func deserialize<D: Deserializer>(deserializer: D) throws -> ViewModel {
        try deserializer.increase_container_depth()
        let status = try LogN.StatusKey.deserialize(deserializer: deserializer)
        let pendingSyncCount = try deserializer.deserialize_u32()
        let isSyncing = try deserializer.deserialize_bool()
        let isFetching = try deserializer.deserialize_bool()
        let isAuthenticating = try deserializer.deserialize_bool()
        let hasAccessToken = try deserializer.deserialize_bool()
        let hasSession = try deserializer.deserialize_bool()
        let isOfflineSession = try deserializer.deserialize_bool()
        let trailFromBundle = try deserializer.deserialize_bool()
        let trailGeneratedAt = try deserializer.deserialize_str()
        let matchLeft = try deserializer.deserialize_bool()
        let isGuest = try deserializer.deserialize_bool()
        let locale = try deserializer.deserialize_str()
        let challenges = try deserializeArray(deserializer: deserializer) { deserializer in
            try LogN.Challenge.deserialize(deserializer: deserializer)
        }
        let nodes = try deserializeArray(deserializer: deserializer) { deserializer in
            try LogN.SkillNode.deserialize(deserializer: deserializer)
        }
        let otpEmail = try deserializer.deserialize_str()
        let accountEmail = try deserializer.deserialize_str()
        let otpVerified = try deserializer.deserialize_bool()
        let globalXp = try deserializer.deserialize_i32()
        let bugsFound = try deserializer.deserialize_i32()
        let dryRunsCompleted = try deserializer.deserialize_i32()
        let level = try deserializer.deserialize_i32()
        let xpIntoLevel = try deserializer.deserialize_i32()
        let xpForLevel = try deserializer.deserialize_i32()
        let xpToNextLevel = try deserializer.deserialize_i32()
        let challengesCompleted = try deserializer.deserialize_i32()
        let balloonsUp = try deserializer.deserialize_i32()
        let justLoggedOut = try deserializer.deserialize_bool()
        let passwordResetDone = try deserializer.deserialize_bool()
        let displayName = try deserializer.deserialize_str()
        let matchView = try LogN.MatchViewModel.deserialize(deserializer: deserializer)
        let contestName = try deserializer.deserialize_str()
        let standingsGlobal = try deserializeArray(deserializer: deserializer) { deserializer in
            try LogN.StandingRow.deserialize(deserializer: deserializer)
        }
        let standingsHome = try deserializeArray(deserializer: deserializer) { deserializer in
            try LogN.StandingRow.deserialize(deserializer: deserializer)
        }
        let userStanding = try LogN.StandingRow.deserialize(deserializer: deserializer)
        let scoreboard = try deserializeArray(deserializer: deserializer) { deserializer in
            try LogN.ScoreboardRow.deserialize(deserializer: deserializer)
        }
        let standingsAreSample = try deserializer.deserialize_bool()
        let authCooldownSeconds = try deserializer.deserialize_u32()
        let resendCooldownSeconds = try deserializer.deserialize_u32()
        let legalVersionsReady = try deserializer.deserialize_bool()
        let minAge = try deserializer.deserialize_u32()
        let deletionPurgeAfter = try deserializer.deserialize_i64()
        let accountRestoredNotice = try deserializer.deserialize_bool()
        let socialSignupRequired = try deserializer.deserialize_bool()
        let analyticsEnabled = try deserializer.deserialize_bool()
        let boot = try LogN.BootViewModel.deserialize(deserializer: deserializer)
        let termsUpdate = try deserializeOption(deserializer: deserializer) { deserializer in
            try LogN.TermsUpdateViewModel.deserialize(deserializer: deserializer)
        }
        let termsNotice = try deserializer.deserialize_bool()
        let resumeEmail = try deserializer.deserialize_str()
        let accountUserId = try deserializer.deserialize_str()
        let tracks = try deserializeArray(deserializer: deserializer) { deserializer in
            try LogN.TrackView.deserialize(deserializer: deserializer)
        }
        let currentTrack = try LogN.TrackView.deserialize(deserializer: deserializer)
        let catalogBadge = try LogN.CatalogBadge.deserialize(deserializer: deserializer)
        let showOnboarding = try deserializer.deserialize_bool()
        let purchaseFlow = try LogN.PurchaseFlowView.deserialize(deserializer: deserializer)
        let restoreResult = try LogN.RestoreResultView.deserialize(deserializer: deserializer)
        let sampleOffer = try LogN.SampleOfferView.deserialize(deserializer: deserializer)
        let purchasesToFinish = try deserializeArray(deserializer: deserializer) { deserializer in
            try deserializer.deserialize_str()
        }
        let purchaseInFlight = try deserializer.deserialize_bool()
        try deserializer.decrease_container_depth()
        return ViewModel(status: status, pendingSyncCount: pendingSyncCount, isSyncing: isSyncing, isFetching: isFetching, isAuthenticating: isAuthenticating, hasAccessToken: hasAccessToken, hasSession: hasSession, isOfflineSession: isOfflineSession, trailFromBundle: trailFromBundle, trailGeneratedAt: trailGeneratedAt, matchLeft: matchLeft, isGuest: isGuest, locale: locale, challenges: challenges, nodes: nodes, otpEmail: otpEmail, accountEmail: accountEmail, otpVerified: otpVerified, globalXp: globalXp, bugsFound: bugsFound, dryRunsCompleted: dryRunsCompleted, level: level, xpIntoLevel: xpIntoLevel, xpForLevel: xpForLevel, xpToNextLevel: xpToNextLevel, challengesCompleted: challengesCompleted, balloonsUp: balloonsUp, justLoggedOut: justLoggedOut, passwordResetDone: passwordResetDone, displayName: displayName, matchView: matchView, contestName: contestName, standingsGlobal: standingsGlobal, standingsHome: standingsHome, userStanding: userStanding, scoreboard: scoreboard, standingsAreSample: standingsAreSample, authCooldownSeconds: authCooldownSeconds, resendCooldownSeconds: resendCooldownSeconds, legalVersionsReady: legalVersionsReady, minAge: minAge, deletionPurgeAfter: deletionPurgeAfter, accountRestoredNotice: accountRestoredNotice, socialSignupRequired: socialSignupRequired, analyticsEnabled: analyticsEnabled, boot: boot, termsUpdate: termsUpdate, termsNotice: termsNotice, resumeEmail: resumeEmail, accountUserId: accountUserId, tracks: tracks, currentTrack: currentTrack, catalogBadge: catalogBadge, showOnboarding: showOnboarding, purchaseFlow: purchaseFlow, restoreResult: restoreResult, sampleOffer: sampleOffer, purchasesToFinish: purchasesToFinish, purchaseInFlight: purchaseInFlight)
    }

    public static func bincodeDeserialize(input: [UInt8]) throws -> ViewModel {
        let deserializer = BincodeDeserializer.init(input: input);
        let obj = try deserialize(deserializer: deserializer)
        if deserializer.get_buffer_offset() < input.count {
            throw DeserializationError.invalidInput(issue: "Some input bytes were not read")
        }
        return obj
    }
}

/// Uma variável do painel de watch do DRY_RUN — nome e valor no estado inicial.
/// Não entrega a resposta: diz *o que* acompanhar durante o trace.
public struct WatchVariable: Hashable, Equatable {
    public var name: String
    public var value: String

    public init(name: String, value: String) {
        self.name = name
        self.value = value
    }

    public func serialize<S: Serializer>(serializer: S) throws {
        try serializer.increase_container_depth()
        try serializer.serialize_str(value: self.name)
        try serializer.serialize_str(value: self.value)
        try serializer.decrease_container_depth()
    }

    public func bincodeSerialize() throws -> [UInt8] {
        let serializer = BincodeSerializer.init();
        try self.serialize(serializer: serializer)
        return serializer.get_bytes()
    }

    public static func deserialize<D: Deserializer>(deserializer: D) throws -> WatchVariable {
        try deserializer.increase_container_depth()
        let name = try deserializer.deserialize_str()
        let value = try deserializer.deserialize_str()
        try deserializer.decrease_container_depth()
        return WatchVariable(name: name, value: value)
    }

    public static func bincodeDeserialize(input: [UInt8]) throws -> WatchVariable {
        let deserializer = BincodeDeserializer.init(input: input);
        let obj = try deserialize(deserializer: deserializer)
        if deserializer.get_buffer_offset() < input.count {
            throw DeserializationError.invalidInput(issue: "Some input bytes were not read")
        }
        return obj
    }
}
