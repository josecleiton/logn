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

public struct Challenge: Hashable, Equatable {
    public var id: String
    public var nodeId: String
    public var templateType: String
    public var payload: ChallengePayload
    /// De onde o desafio veio, quando não foi escrito para o LogN — hoje só `FARIAS`,
    /// a origem registrada no conteúdo. Vazio é o caso comum, e o
    /// `default` mantém compatível o JSON gravado antes de a coluna existir.
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
    public var description: TrapKey
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

    public init(title: String, description: TrapKey, codeLines: [String], options: [String]?, correctOptions: [String]?, watchVariables: [WatchVariable]?, watchNote: String?, seconds: Int32?) {
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
        try self.description.serialize(serializer: serializer)
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
        let description = try LogN.TrapKey.deserialize(deserializer: deserializer)
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
        case .time(let x):
            try serializer.serialize_variant_index(value: 5)
            try x.serialize(serializer: serializer)
        case .storeReview(let x):
            try serializer.serialize_variant_index(value: 6)
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
            let x = try LogN.TimeRequest.deserialize(deserializer: deserializer)
            try deserializer.decrease_container_depth()
            return .time(x)
        case 6:
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
    case login(email: String, passwordHash: String)
    case loginCompleted(HttpResult)
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
    case syncNow
    case syncAndLogout
    case restoreOfflineQueue
    case offlineQueueRestored(KeyValueResult)
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
    case register(email: String, password: String, otp: String, ageConfirmed: Bool, legalAcceptances: [String])
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
        case .login(let email, let passwordHash):
            try serializer.serialize_variant_index(value: 4)
            try serializer.serialize_str(value: email)
            try serializer.serialize_str(value: passwordHash)
        case .loginCompleted(let x):
            try serializer.serialize_variant_index(value: 5)
            try x.serialize(serializer: serializer)
        case .continueAsGuest:
            try serializer.serialize_variant_index(value: 6)
        case .tokenStored(let x):
            try serializer.serialize_variant_index(value: 7)
            try x.serialize(serializer: serializer)
        case .accountEmailStored(let x):
            try serializer.serialize_variant_index(value: 8)
            try x.serialize(serializer: serializer)
        case .accountEmailRead(let x):
            try serializer.serialize_variant_index(value: 9)
            try x.serialize(serializer: serializer)
        case .attemptRefresh:
            try serializer.serialize_variant_index(value: 10)
        case .tokenRead(let x):
            try serializer.serialize_variant_index(value: 11)
            try x.serialize(serializer: serializer)
        case .tokenCleared(let x):
            try serializer.serialize_variant_index(value: 12)
            try x.serialize(serializer: serializer)
        case .refreshCompleted(let x):
            try serializer.serialize_variant_index(value: 13)
            try x.serialize(serializer: serializer)
        case .logout:
            try serializer.serialize_variant_index(value: 14)
        case .fetchChallenges:
            try serializer.serialize_variant_index(value: 15)
        case .fetchNodes:
            try serializer.serialize_variant_index(value: 16)
        case .fetchProgress:
            try serializer.serialize_variant_index(value: 17)
        case .progressFetched(let x):
            try serializer.serialize_variant_index(value: 18)
            try x.serialize(serializer: serializer)
        case .nodesFetched(let x):
            try serializer.serialize_variant_index(value: 19)
            try x.serialize(serializer: serializer)
        case .challengesFetched(let x):
            try serializer.serialize_variant_index(value: 20)
            try x.serialize(serializer: serializer)
        case .syncNow:
            try serializer.serialize_variant_index(value: 21)
        case .syncAndLogout:
            try serializer.serialize_variant_index(value: 22)
        case .restoreOfflineQueue:
            try serializer.serialize_variant_index(value: 23)
        case .offlineQueueRestored(let x):
            try serializer.serialize_variant_index(value: 24)
            try x.serialize(serializer: serializer)
        case .bundledTrailLoaded(let json):
            try serializer.serialize_variant_index(value: 25)
            try serializer.serialize_str(value: json)
        case .leaveMatch:
            try serializer.serialize_variant_index(value: 26)
        case .confirmLeaveMatch:
            try serializer.serialize_variant_index(value: 27)
        case .cancelLeaveMatch:
            try serializer.serialize_variant_index(value: 28)
        case .openOriginSheet:
            try serializer.serialize_variant_index(value: 29)
        case .closeOriginSheet:
            try serializer.serialize_variant_index(value: 30)
        case .originsSeenRestored(let x):
            try serializer.serialize_variant_index(value: 31)
            try x.serialize(serializer: serializer)
        case .originsSeenStored(let x):
            try serializer.serialize_variant_index(value: 32)
            try x.serialize(serializer: serializer)
        case .dismissPasswordReset:
            try serializer.serialize_variant_index(value: 33)
        case .tick(let now):
            try serializer.serialize_variant_index(value: 34)
            try serializer.serialize_i64(value: now)
        case .cooldownClock(let now):
            try serializer.serialize_variant_index(value: 35)
            try serializer.serialize_i64(value: now)
        case .cooldownElapsed:
            try serializer.serialize_variant_index(value: 36)
        case .sessionExpiryStored(let x):
            try serializer.serialize_variant_index(value: 37)
            try x.serialize(serializer: serializer)
        case .offlineSessionChecked(let x):
            try serializer.serialize_variant_index(value: 38)
            try x.serialize(serializer: serializer)
        case .snapshotSaved(let x):
            try serializer.serialize_variant_index(value: 39)
            try x.serialize(serializer: serializer)
        case .snapshotRestored(let x):
            try serializer.serialize_variant_index(value: 40)
            try x.serialize(serializer: serializer)
        case .rotatedTokenStored(let x):
            try serializer.serialize_variant_index(value: 41)
            try x.serialize(serializer: serializer)
        case .attemptRefreshDone:
            try serializer.serialize_variant_index(value: 42)
        case .syncCompleted(let x):
            try serializer.serialize_variant_index(value: 43)
            try x.serialize(serializer: serializer)
        case .requestOtp(let email, let purpose):
            try serializer.serialize_variant_index(value: 44)
            try serializer.serialize_str(value: email)
            try serializer.serialize_str(value: purpose)
        case .otpRequested(let x):
            try serializer.serialize_variant_index(value: 45)
            try x.serialize(serializer: serializer)
        case .verifyOtp(let email, let code, let purpose):
            try serializer.serialize_variant_index(value: 46)
            try serializer.serialize_str(value: email)
            try serializer.serialize_str(value: code)
            try serializer.serialize_str(value: purpose)
        case .otpVerified(let x):
            try serializer.serialize_variant_index(value: 47)
            try x.serialize(serializer: serializer)
        case .register(let email, let password, let otp, let ageConfirmed, let legalAcceptances):
            try serializer.serialize_variant_index(value: 48)
            try serializer.serialize_str(value: email)
            try serializer.serialize_str(value: password)
            try serializer.serialize_str(value: otp)
            try serializer.serialize_bool(value: ageConfirmed)
            try serializeArray(value: legalAcceptances, serializer: serializer) { item, serializer in
                try serializer.serialize_str(value: item)
            }
        case .registerCompleted(let x):
            try serializer.serialize_variant_index(value: 49)
            try x.serialize(serializer: serializer)
        case .resetPassword(let email, let newPassword, let otp):
            try serializer.serialize_variant_index(value: 50)
            try serializer.serialize_str(value: email)
            try serializer.serialize_str(value: newPassword)
            try serializer.serialize_str(value: otp)
        case .resetPasswordCompleted(let x):
            try serializer.serialize_variant_index(value: 51)
            try x.serialize(serializer: serializer)
        case .queueSavedForSync(let x):
            try serializer.serialize_variant_index(value: 52)
            try x.serialize(serializer: serializer)
        case .startMatch(let nodeId):
            try serializer.serialize_variant_index(value: 53)
            try serializer.serialize_str(value: nodeId)
        case .matchSelectLine(let line):
            try serializer.serialize_variant_index(value: 54)
            try serializer.serialize_i32(value: line)
        case .matchSetAnswer(let answer):
            try serializer.serialize_variant_index(value: 55)
            try serializer.serialize_str(value: answer)
        case .matchSetDropTime(let value):
            try serializer.serialize_variant_index(value: 56)
            try serializer.serialize_str(value: value)
        case .matchSetDropSpace(let value):
            try serializer.serialize_variant_index(value: 57)
            try serializer.serialize_str(value: value)
        case .matchToggleTag(let tag):
            try serializer.serialize_variant_index(value: 58)
            try serializer.serialize_str(value: tag)
        case .matchSetOutput(let value):
            try serializer.serialize_variant_index(value: 59)
            try serializer.serialize_str(value: value)
        case .matchSubmit(let timestamp):
            try serializer.serialize_variant_index(value: 60)
            try serializer.serialize_i64(value: timestamp)
        case .matchDismissTrap:
            try serializer.serialize_variant_index(value: 61)
        case .matchTimerTick:
            try serializer.serialize_variant_index(value: 62)
        case .undoLogout:
            try serializer.serialize_variant_index(value: 63)
        case .dismissLogoutNotice:
            try serializer.serialize_variant_index(value: 64)
        case .logoutUndone(let x):
            try serializer.serialize_variant_index(value: 65)
            try x.serialize(serializer: serializer)
        case .matchAbandonedAt(let solved, let now):
            try serializer.serialize_variant_index(value: 66)
            try serializer.serialize_i32(value: solved)
            try serializer.serialize_i64(value: now)
        case .matchReportClosed:
            try serializer.serialize_variant_index(value: 67)
        case .reviewMilestonesFired(let x):
            try serializer.serialize_variant_index(value: 68)
            try x.serialize(serializer: serializer)
        case .reviewMilestonesRestored(let x):
            try serializer.serialize_variant_index(value: 69)
            try x.serialize(serializer: serializer)
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
            let passwordHash = try deserializer.deserialize_str()
            try deserializer.decrease_container_depth()
            return .login(email: email, passwordHash: passwordHash)
        case 5:
            let x = try LogN.HttpResult.deserialize(deserializer: deserializer)
            try deserializer.decrease_container_depth()
            return .loginCompleted(x)
        case 6:
            try deserializer.decrease_container_depth()
            return .continueAsGuest
        case 7:
            let x = try LogN.KeyValueResult.deserialize(deserializer: deserializer)
            try deserializer.decrease_container_depth()
            return .tokenStored(x)
        case 8:
            let x = try LogN.KeyValueResult.deserialize(deserializer: deserializer)
            try deserializer.decrease_container_depth()
            return .accountEmailStored(x)
        case 9:
            let x = try LogN.KeyValueResult.deserialize(deserializer: deserializer)
            try deserializer.decrease_container_depth()
            return .accountEmailRead(x)
        case 10:
            try deserializer.decrease_container_depth()
            return .attemptRefresh
        case 11:
            let x = try LogN.KeyValueResult.deserialize(deserializer: deserializer)
            try deserializer.decrease_container_depth()
            return .tokenRead(x)
        case 12:
            let x = try LogN.KeyValueResult.deserialize(deserializer: deserializer)
            try deserializer.decrease_container_depth()
            return .tokenCleared(x)
        case 13:
            let x = try LogN.HttpResult.deserialize(deserializer: deserializer)
            try deserializer.decrease_container_depth()
            return .refreshCompleted(x)
        case 14:
            try deserializer.decrease_container_depth()
            return .logout
        case 15:
            try deserializer.decrease_container_depth()
            return .fetchChallenges
        case 16:
            try deserializer.decrease_container_depth()
            return .fetchNodes
        case 17:
            try deserializer.decrease_container_depth()
            return .fetchProgress
        case 18:
            let x = try LogN.HttpResult.deserialize(deserializer: deserializer)
            try deserializer.decrease_container_depth()
            return .progressFetched(x)
        case 19:
            let x = try LogN.HttpResult.deserialize(deserializer: deserializer)
            try deserializer.decrease_container_depth()
            return .nodesFetched(x)
        case 20:
            let x = try LogN.HttpResult.deserialize(deserializer: deserializer)
            try deserializer.decrease_container_depth()
            return .challengesFetched(x)
        case 21:
            try deserializer.decrease_container_depth()
            return .syncNow
        case 22:
            try deserializer.decrease_container_depth()
            return .syncAndLogout
        case 23:
            try deserializer.decrease_container_depth()
            return .restoreOfflineQueue
        case 24:
            let x = try LogN.KeyValueResult.deserialize(deserializer: deserializer)
            try deserializer.decrease_container_depth()
            return .offlineQueueRestored(x)
        case 25:
            let json = try deserializer.deserialize_str()
            try deserializer.decrease_container_depth()
            return .bundledTrailLoaded(json: json)
        case 26:
            try deserializer.decrease_container_depth()
            return .leaveMatch
        case 27:
            try deserializer.decrease_container_depth()
            return .confirmLeaveMatch
        case 28:
            try deserializer.decrease_container_depth()
            return .cancelLeaveMatch
        case 29:
            try deserializer.decrease_container_depth()
            return .openOriginSheet
        case 30:
            try deserializer.decrease_container_depth()
            return .closeOriginSheet
        case 31:
            let x = try LogN.KeyValueResult.deserialize(deserializer: deserializer)
            try deserializer.decrease_container_depth()
            return .originsSeenRestored(x)
        case 32:
            let x = try LogN.KeyValueResult.deserialize(deserializer: deserializer)
            try deserializer.decrease_container_depth()
            return .originsSeenStored(x)
        case 33:
            try deserializer.decrease_container_depth()
            return .dismissPasswordReset
        case 34:
            let now = try deserializer.deserialize_i64()
            try deserializer.decrease_container_depth()
            return .tick(now: now)
        case 35:
            let now = try deserializer.deserialize_i64()
            try deserializer.decrease_container_depth()
            return .cooldownClock(now: now)
        case 36:
            try deserializer.decrease_container_depth()
            return .cooldownElapsed
        case 37:
            let x = try LogN.KeyValueResult.deserialize(deserializer: deserializer)
            try deserializer.decrease_container_depth()
            return .sessionExpiryStored(x)
        case 38:
            let x = try LogN.KeyValueResult.deserialize(deserializer: deserializer)
            try deserializer.decrease_container_depth()
            return .offlineSessionChecked(x)
        case 39:
            let x = try LogN.KeyValueResult.deserialize(deserializer: deserializer)
            try deserializer.decrease_container_depth()
            return .snapshotSaved(x)
        case 40:
            let x = try LogN.KeyValueResult.deserialize(deserializer: deserializer)
            try deserializer.decrease_container_depth()
            return .snapshotRestored(x)
        case 41:
            let x = try LogN.KeyValueResult.deserialize(deserializer: deserializer)
            try deserializer.decrease_container_depth()
            return .rotatedTokenStored(x)
        case 42:
            try deserializer.decrease_container_depth()
            return .attemptRefreshDone
        case 43:
            let x = try LogN.HttpResult.deserialize(deserializer: deserializer)
            try deserializer.decrease_container_depth()
            return .syncCompleted(x)
        case 44:
            let email = try deserializer.deserialize_str()
            let purpose = try deserializer.deserialize_str()
            try deserializer.decrease_container_depth()
            return .requestOtp(email: email, purpose: purpose)
        case 45:
            let x = try LogN.HttpResult.deserialize(deserializer: deserializer)
            try deserializer.decrease_container_depth()
            return .otpRequested(x)
        case 46:
            let email = try deserializer.deserialize_str()
            let code = try deserializer.deserialize_str()
            let purpose = try deserializer.deserialize_str()
            try deserializer.decrease_container_depth()
            return .verifyOtp(email: email, code: code, purpose: purpose)
        case 47:
            let x = try LogN.HttpResult.deserialize(deserializer: deserializer)
            try deserializer.decrease_container_depth()
            return .otpVerified(x)
        case 48:
            let email = try deserializer.deserialize_str()
            let password = try deserializer.deserialize_str()
            let otp = try deserializer.deserialize_str()
            let ageConfirmed = try deserializer.deserialize_bool()
            let legalAcceptances = try deserializeArray(deserializer: deserializer) { deserializer in
                try deserializer.deserialize_str()
            }
            try deserializer.decrease_container_depth()
            return .register(email: email, password: password, otp: otp, ageConfirmed: ageConfirmed, legalAcceptances: legalAcceptances)
        case 49:
            let x = try LogN.HttpResult.deserialize(deserializer: deserializer)
            try deserializer.decrease_container_depth()
            return .registerCompleted(x)
        case 50:
            let email = try deserializer.deserialize_str()
            let newPassword = try deserializer.deserialize_str()
            let otp = try deserializer.deserialize_str()
            try deserializer.decrease_container_depth()
            return .resetPassword(email: email, newPassword: newPassword, otp: otp)
        case 51:
            let x = try LogN.HttpResult.deserialize(deserializer: deserializer)
            try deserializer.decrease_container_depth()
            return .resetPasswordCompleted(x)
        case 52:
            let x = try LogN.KeyValueResult.deserialize(deserializer: deserializer)
            try deserializer.decrease_container_depth()
            return .queueSavedForSync(x)
        case 53:
            let nodeId = try deserializer.deserialize_str()
            try deserializer.decrease_container_depth()
            return .startMatch(nodeId: nodeId)
        case 54:
            let line = try deserializer.deserialize_i32()
            try deserializer.decrease_container_depth()
            return .matchSelectLine(line: line)
        case 55:
            let answer = try deserializer.deserialize_str()
            try deserializer.decrease_container_depth()
            return .matchSetAnswer(answer: answer)
        case 56:
            let value = try deserializer.deserialize_str()
            try deserializer.decrease_container_depth()
            return .matchSetDropTime(value: value)
        case 57:
            let value = try deserializer.deserialize_str()
            try deserializer.decrease_container_depth()
            return .matchSetDropSpace(value: value)
        case 58:
            let tag = try deserializer.deserialize_str()
            try deserializer.decrease_container_depth()
            return .matchToggleTag(tag: tag)
        case 59:
            let value = try deserializer.deserialize_str()
            try deserializer.decrease_container_depth()
            return .matchSetOutput(value: value)
        case 60:
            let timestamp = try deserializer.deserialize_i64()
            try deserializer.decrease_container_depth()
            return .matchSubmit(timestamp: timestamp)
        case 61:
            try deserializer.decrease_container_depth()
            return .matchDismissTrap
        case 62:
            try deserializer.decrease_container_depth()
            return .matchTimerTick
        case 63:
            try deserializer.decrease_container_depth()
            return .undoLogout
        case 64:
            try deserializer.decrease_container_depth()
            return .dismissLogoutNotice
        case 65:
            let x = try LogN.KeyValueResult.deserialize(deserializer: deserializer)
            try deserializer.decrease_container_depth()
            return .logoutUndone(x)
        case 66:
            let solved = try deserializer.deserialize_i32()
            let now = try deserializer.deserialize_i64()
            try deserializer.decrease_container_depth()
            return .matchAbandonedAt(solved: solved, now: now)
        case 67:
            try deserializer.decrease_container_depth()
            return .matchReportClosed
        case 68:
            let x = try LogN.KeyValueResult.deserialize(deserializer: deserializer)
            try deserializer.decrease_container_depth()
            return .reviewMilestonesFired(x)
        case 69:
            let x = try LogN.KeyValueResult.deserialize(deserializer: deserializer)
            try deserializer.decrease_container_depth()
            return .reviewMilestonesRestored(x)
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

/// Um erro da sessão, guardado para a revisão do relatório pós-partida.
/// Um por problema errado — o relatório lista todos, não só o último.
public struct MatchError: Hashable, Equatable {
    public var letter: String
    public var title: String
    /// Sigla do juiz: `WA`, `TLE`, …
    public var verdict: String
    /// O que o jogador respondeu, já em texto legível.
    public var givenAnswer: String
    public var explanation: String

    public init(letter: String, title: String, verdict: String, givenAnswer: String, explanation: String) {
        self.letter = letter
        self.title = title
        self.verdict = verdict
        self.givenAnswer = givenAnswer
        self.explanation = explanation
    }

    public func serialize<S: Serializer>(serializer: S) throws {
        try serializer.increase_container_depth()
        try serializer.serialize_str(value: self.letter)
        try serializer.serialize_str(value: self.title)
        try serializer.serialize_str(value: self.verdict)
        try serializer.serialize_str(value: self.givenAnswer)
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
        let explanation = try deserializer.deserialize_str()
        try deserializer.decrease_container_depth()
        return MatchError(letter: letter, title: title, verdict: verdict, givenAnswer: givenAnswer, explanation: explanation)
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
    public var currentDescription: TrapKey
    public var currentTemplateType: String
    public var currentCodeLines: [String]
    public var currentOptions: [String]
    public var maxSelections: Int32
    /// Origem do problema atual, vazia quando ele nasceu aqui.
    public var currentOrigin: String
    /// Origem cujo cartão de homenagem está aberto. Vazia quando não há cartão.
    public var originSheet: String
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
    public var selectedTags: [String]
    public var predictedOutput: String
    public var watchVariables: [WatchVariable]
    public var watchNote: String
    public var lastVerdict: String
    public var errors: [MatchError]
    public var hasTrap: Bool
    public var trapCategory: String
    public var trapTitle: String
    public var trapExplanation: String

    public init(isActive: Bool, currentLetter: String, currentTitle: String, currentDescription: TrapKey, currentTemplateType: String, currentCodeLines: [String], currentOptions: [String], maxSelections: Int32, currentOrigin: String, originSheet: String, originSheetPaused: Bool, leavePending: Bool, solvedSoFar: Int32, lives: Int32, maxLives: Int32, penaltyMinutes: Int32, contestSeconds: Int32, questionSeconds: Int32, isFrozen: Bool, totalProblems: Int32, solvedCount: Int32, balloonStates: [BalloonState], xpEarned: Int32, selectedLine: Int32, answerString: String, dropTime: String, dropSpace: String, selectedTags: [String], predictedOutput: String, watchVariables: [WatchVariable], watchNote: String, lastVerdict: String, errors: [MatchError], hasTrap: Bool, trapCategory: String, trapTitle: String, trapExplanation: String) {
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
        self.selectedTags = selectedTags
        self.predictedOutput = predictedOutput
        self.watchVariables = watchVariables
        self.watchNote = watchNote
        self.lastVerdict = lastVerdict
        self.errors = errors
        self.hasTrap = hasTrap
        self.trapCategory = trapCategory
        self.trapTitle = trapTitle
        self.trapExplanation = trapExplanation
    }

    public func serialize<S: Serializer>(serializer: S) throws {
        try serializer.increase_container_depth()
        try serializer.serialize_bool(value: self.isActive)
        try serializer.serialize_str(value: self.currentLetter)
        try serializer.serialize_str(value: self.currentTitle)
        try self.currentDescription.serialize(serializer: serializer)
        try serializer.serialize_str(value: self.currentTemplateType)
        try serializeArray(value: self.currentCodeLines, serializer: serializer) { item, serializer in
            try serializer.serialize_str(value: item)
        }
        try serializeArray(value: self.currentOptions, serializer: serializer) { item, serializer in
            try serializer.serialize_str(value: item)
        }
        try serializer.serialize_i32(value: self.maxSelections)
        try serializer.serialize_str(value: self.currentOrigin)
        try serializer.serialize_str(value: self.originSheet)
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
        try serializer.serialize_str(value: self.trapCategory)
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
        let currentDescription = try LogN.TrapKey.deserialize(deserializer: deserializer)
        let currentTemplateType = try deserializer.deserialize_str()
        let currentCodeLines = try deserializeArray(deserializer: deserializer) { deserializer in
            try deserializer.deserialize_str()
        }
        let currentOptions = try deserializeArray(deserializer: deserializer) { deserializer in
            try deserializer.deserialize_str()
        }
        let maxSelections = try deserializer.deserialize_i32()
        let currentOrigin = try deserializer.deserialize_str()
        let originSheet = try deserializer.deserialize_str()
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
        let trapCategory = try deserializer.deserialize_str()
        let trapTitle = try deserializer.deserialize_str()
        let trapExplanation = try deserializer.deserialize_str()
        try deserializer.decrease_container_depth()
        return MatchViewModel(isActive: isActive, currentLetter: currentLetter, currentTitle: currentTitle, currentDescription: currentDescription, currentTemplateType: currentTemplateType, currentCodeLines: currentCodeLines, currentOptions: currentOptions, maxSelections: maxSelections, currentOrigin: currentOrigin, originSheet: originSheet, originSheetPaused: originSheetPaused, leavePending: leavePending, solvedSoFar: solvedSoFar, lives: lives, maxLives: maxLives, penaltyMinutes: penaltyMinutes, contestSeconds: contestSeconds, questionSeconds: questionSeconds, isFrozen: isFrozen, totalProblems: totalProblems, solvedCount: solvedCount, balloonStates: balloonStates, xpEarned: xpEarned, selectedLine: selectedLine, answerString: answerString, dropTime: dropTime, dropSpace: dropSpace, selectedTags: selectedTags, predictedOutput: predictedOutput, watchVariables: watchVariables, watchNote: watchNote, lastVerdict: lastVerdict, errors: errors, hasTrap: hasTrap, trapCategory: trapCategory, trapTitle: trapTitle, trapExplanation: trapExplanation)
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

    public func serialize<S: Serializer>(serializer: S) throws {
        try serializer.increase_container_depth()
        switch self {
        case .locked:
            try serializer.serialize_variant_index(value: 0)
        case .active:
            try serializer.serialize_variant_index(value: 1)
        case .completed:
            try serializer.serialize_variant_index(value: 2)
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
    /// Derivado em `view()` a partir do XP e do DAG, nunca persistido: o Go não manda
    /// este campo, e sem o `default` a lista inteira falhava no `serde` — o usuário
    /// autenticado via uma árvore vazia sem erro nenhum aparecer.
    public var status: NodeStatus
    /// Um por problema do nó, na ordem das letras: `true` quando ele já rendeu XP.
    /// Derivado em `view()`, como `status`. É o que a trilha conta (`3/5`) e o que o
    /// sheet desenha balão a balão.
    public var problemsSolved: [Bool]

    public init(id: String, name: String, description: String, row: Int32, column: Int32, requiredXp: Int32, prerequisites: [String], status: NodeStatus, problemsSolved: [Bool]) {
        self.id = id
        self.name = name
        self.description = description
        self.row = row
        self.column = column
        self.requiredXp = requiredXp
        self.prerequisites = prerequisites
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
        let status = try LogN.NodeStatus.deserialize(deserializer: deserializer)
        let problemsSolved = try deserializeArray(deserializer: deserializer) { deserializer in
            try deserializer.deserialize_bool()
        }
        try deserializer.decrease_container_depth()
        return SkillNode(id: id, name: name, description: description, row: row, column: column, requiredXp: requiredXp, prerequisites: prerequisites, status: status, problemsSolved: problemsSolved)
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

/// O que o Core tem a dizer ao jogador, como **chave**, não como frase.
/// 
/// O Core escrevia a frase pronta em `status`, e ela vazava para a tela: a de login
/// abria com "Logged out successfully", em inglês, no vermelho de erro, num app em
/// português. A cópia vive em `i18n/locales/`; o cliente resolve a chave.
/// 
/// `Silent` é o estado normal — a maior parte do que acontece não precisa ser narrada.
indirect public enum TrapKey: Hashable, Equatable {
    case findTheBug
    case completeTheLine
    case finalValueOfAcc
    case complexity
    case findTheCrash
    case nullTerminatedString
    case dynamicSubarrays
    case twoPointers

    public func serialize<S: Serializer>(serializer: S) throws {
        try serializer.increase_container_depth()
        switch self {
        case .findTheBug:
            try serializer.serialize_variant_index(value: 0)
        case .completeTheLine:
            try serializer.serialize_variant_index(value: 1)
        case .finalValueOfAcc:
            try serializer.serialize_variant_index(value: 2)
        case .complexity:
            try serializer.serialize_variant_index(value: 3)
        case .findTheCrash:
            try serializer.serialize_variant_index(value: 4)
        case .nullTerminatedString:
            try serializer.serialize_variant_index(value: 5)
        case .dynamicSubarrays:
            try serializer.serialize_variant_index(value: 6)
        case .twoPointers:
            try serializer.serialize_variant_index(value: 7)
        }
        try serializer.decrease_container_depth()
    }

    public func bincodeSerialize() throws -> [UInt8] {
        let serializer = BincodeSerializer.init();
        try self.serialize(serializer: serializer)
        return serializer.get_bytes()
    }

    public static func deserialize<D: Deserializer>(deserializer: D) throws -> TrapKey {
        let index = try deserializer.deserialize_variant_index()
        try deserializer.increase_container_depth()
        switch index {
        case 0:
            try deserializer.decrease_container_depth()
            return .findTheBug
        case 1:
            try deserializer.decrease_container_depth()
            return .completeTheLine
        case 2:
            try deserializer.decrease_container_depth()
            return .finalValueOfAcc
        case 3:
            try deserializer.decrease_container_depth()
            return .complexity
        case 4:
            try deserializer.decrease_container_depth()
            return .findTheCrash
        case 5:
            try deserializer.decrease_container_depth()
            return .nullTerminatedString
        case 6:
            try deserializer.decrease_container_depth()
            return .dynamicSubarrays
        case 7:
            try deserializer.decrease_container_depth()
            return .twoPointers
        default: throw DeserializationError.invalidInput(issue: "Unknown variant index for TrapKey: \(index)")
        }
    }

    public static func bincodeDeserialize(input: [UInt8]) throws -> TrapKey {
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

    public init(status: StatusKey, pendingSyncCount: UInt32, isSyncing: Bool, isFetching: Bool, isAuthenticating: Bool, hasAccessToken: Bool, hasSession: Bool, isOfflineSession: Bool, trailFromBundle: Bool, trailGeneratedAt: String, matchLeft: Bool, isGuest: Bool, locale: String, challenges: [Challenge], nodes: [SkillNode], otpEmail: String, accountEmail: String, otpVerified: Bool, globalXp: Int32, bugsFound: Int32, dryRunsCompleted: Int32, level: Int32, xpIntoLevel: Int32, xpForLevel: Int32, xpToNextLevel: Int32, challengesCompleted: Int32, balloonsUp: Int32, justLoggedOut: Bool, passwordResetDone: Bool, displayName: String, matchView: MatchViewModel, contestName: String, standingsGlobal: [StandingRow], standingsHome: [StandingRow], userStanding: StandingRow, scoreboard: [ScoreboardRow], standingsAreSample: Bool, authCooldownSeconds: UInt32, resendCooldownSeconds: UInt32) {
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
        try deserializer.decrease_container_depth()
        return ViewModel(status: status, pendingSyncCount: pendingSyncCount, isSyncing: isSyncing, isFetching: isFetching, isAuthenticating: isAuthenticating, hasAccessToken: hasAccessToken, hasSession: hasSession, isOfflineSession: isOfflineSession, trailFromBundle: trailFromBundle, trailGeneratedAt: trailGeneratedAt, matchLeft: matchLeft, isGuest: isGuest, locale: locale, challenges: challenges, nodes: nodes, otpEmail: otpEmail, accountEmail: accountEmail, otpVerified: otpVerified, globalXp: globalXp, bugsFound: bugsFound, dryRunsCompleted: dryRunsCompleted, level: level, xpIntoLevel: xpIntoLevel, xpForLevel: xpForLevel, xpToNextLevel: xpToNextLevel, challengesCompleted: challengesCompleted, balloonsUp: balloonsUp, justLoggedOut: justLoggedOut, passwordResetDone: passwordResetDone, displayName: displayName, matchView: matchView, contestName: contestName, standingsGlobal: standingsGlobal, standingsHome: standingsHome, userStanding: userStanding, scoreboard: scoreboard, standingsAreSample: standingsAreSample, authCooldownSeconds: authCooldownSeconds, resendCooldownSeconds: resendCooldownSeconds)
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
