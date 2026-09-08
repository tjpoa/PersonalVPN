// swift-tools-version: 5.9
import PackageDescription

let package = Package(
    name: "PersonalVPNClient",
    platforms: [.iOS(.v15), .macOS(.v13)],
    products: [.library(name: "PersonalVPNClient", targets: ["PersonalVPNClient"])],
    targets: [
        .target(name: "PersonalVPNClient"),
        .testTarget(name: "PersonalVPNClientTests", dependencies: ["PersonalVPNClient"])
    ]
)
