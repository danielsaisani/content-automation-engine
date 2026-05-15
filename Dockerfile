# ---- Stage 1: Builder ----
    FROM golang:1.24.4-bookworm AS builder

    WORKDIR /build
    
    COPY incubator/go.mod incubator/go.sum ./
    
    RUN go mod download
    
    COPY incubator/ ./
    
    RUN CGO_ENABLED=0 GOOS=linux go build -o bin/content-automation-engine ./cmd/engine/main.go
    
    # ---- Stage 2: Runner ----
    FROM scratch
    
    # Copy certificate authority from build step
    COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
    COPY --from=builder /build/bin/content-automation-engine /content-automation-engine
    
    EXPOSE 8000
    
    CMD ["/content-automation-engine"]