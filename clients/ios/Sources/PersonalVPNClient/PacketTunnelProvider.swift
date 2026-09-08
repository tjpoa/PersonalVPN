#if canImport(NetworkExtension)
import NetworkExtension

/// Network Extension boundary. WireGuardKit is injected by the signed iOS target.
/// The provider fails closed until that engine is configured, never exposing traffic.
public final class PersonalVPNPacketTunnelProvider: NEPacketTunnelProvider {
    public override func startTunnel(options: [String: NSObject]?,
                                     completionHandler: @escaping (Error?) -> Void) {
        completionHandler(PersonalVPNPacketTunnelError.engineUnavailable)
    }

    public override func stopTunnel(with reason: NEProviderStopReason,
                                    completionHandler: @escaping () -> Void) {
        completionHandler()
    }
}

public enum PersonalVPNPacketTunnelError: Error {
    case engineUnavailable
}
#endif
