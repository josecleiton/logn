// swift-tools-version:5.9
import PackageDescription

let package = Package(
    name: "LogNCoreFFI",
    platforms: [
        .iOS(.v16),
    ],
    products: [
        .library(
            name: "LogNCoreFFI",
            targets: ["LogNCoreFFI"]
        ),
    ],
    targets: [
        .binaryTarget(
            name: "LogNCoreFFIFFI",
            path: "LogNCoreFFI.xcframework"
        ),
        .target(
            name: "Serde",
            path: "Sources/Serde"
        ),
        .target(
            name: "LogN",
            dependencies: ["Serde"],
            path: "Sources/LogN"
        ),
        .target(
            name: "App",
            dependencies: ["LogNCoreFFIFFI", "LogN", "Serde"],
            path: "Sources/App"
        ),
        .target(
            name: "LogNCoreFFI",
            dependencies: ["App", "LogN"],
            path: "Sources/LogNCoreFFI"
        ),
    ]
)
