using System.Text.Json;
using System.Text.Json.Serialization;

namespace PersonalVpn.Client;

public sealed record WireGuardPipeCommand(
    [property: JsonPropertyName("version")] int Version,
    [property: JsonPropertyName("operation")] string Operation,
    [property: JsonPropertyName("configuration")] VpnConfiguration? Configuration = null)
{
    private static readonly JsonSerializerOptions JsonOptions = new()
    {
        UnmappedMemberHandling = JsonUnmappedMemberHandling.Disallow
    };

    public void Validate()
    {
        if (Version != 1) throw new InvalidOperationException("Unsupported pipe protocol version");
        if (Operation is not ("start" or "stop" or "status"))
            throw new InvalidOperationException("Unsupported pipe operation");
        if (Operation == "start")
        {
            if (Configuration is null) throw new InvalidOperationException("Start requires configuration");
            WireGuardConfigRenderer.Validate(Configuration);
        }
        else if (Configuration is not null)
            throw new InvalidOperationException("Only start accepts configuration");
    }

    public static WireGuardPipeCommand Parse(string json)
    {
        var command = JsonSerializer.Deserialize<WireGuardPipeCommand>(json, JsonOptions)
            ?? throw new InvalidOperationException("Empty pipe command");
        command.Validate();
        return command;
    }
}
