# One image for every role; the container command picks the role.
FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
COPY migrations ./migrations
COPY api ./api
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/paytrack ./cmd/paytrack

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/paytrack /paytrack
USER nonroot:nonroot
ENTRYPOINT ["/paytrack"]
