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
            name: "LogNCoreFFI",
            dependencies: ["LogNCoreFFIFFI"],
            path: "Sources"
        ),
    ]
)