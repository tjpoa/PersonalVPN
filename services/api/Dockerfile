FROM golang:1.26-alpine@sha256:b6890e35ded5d19118c2bca3d7754dc4e6f694aac2d0aeb92f9807c2879e4230 AS build
WORKDIR /src
RUN apk add --no-cache gcc musl-dev
COPY go.mod go.sum ./
COPY cmd ./cmd
COPY internal ./internal
RUN test -z "$(gofmt -l ./cmd ./internal)" \
    && go vet ./... \
    && go test -race ./... \
    && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/personalvpn-api ./cmd/api \
    && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/personalvpn-reconciler ./cmd/reconciler \
    && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/personalvpn-gateway-agent ./cmd/gateway-agent

FROM scratch AS api
COPY --from=build /out/personalvpn-api /personalvpn-api
EXPOSE 8080
USER 65532:65532
ENTRYPOINT ["/personalvpn-api"]

FROM scratch AS reconciler
COPY --from=build /out/personalvpn-reconciler /personalvpn-reconciler
USER 65532:65532
ENTRYPOINT ["/personalvpn-reconciler"]

FROM scratch AS gateway-agent
COPY --from=build /out/personalvpn-gateway-agent /personalvpn-gateway-agent
USER 65532:65532
ENTRYPOINT ["/personalvpn-gateway-agent"]
