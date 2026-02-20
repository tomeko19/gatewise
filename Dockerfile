FROM golang:1.23-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/gatewise ./cmd/gatewise

FROM gcr.io/distroless/static:nonroot
COPY --from=build /out/gatewise /gatewise
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/gatewise"]

