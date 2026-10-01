FROM golang:1.26-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go generate ./api/...

ARG VERSION=dev
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -X main.version=${VERSION}" -o /out/app ./cmd/app

FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=build /out/app /app

ENV GIN_MODE=release

USER nonroot:nonroot
EXPOSE 8080

ENTRYPOINT ["/app"]
CMD ["serve"]
