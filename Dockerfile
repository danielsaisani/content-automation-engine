# ---- Stage 1: Builder ----
    FROM golang:1.26.11-bookworm AS builder

    WORKDIR /build
    
    COPY incubator/go.mod incubator/go.sum ./
    
    RUN go mod download
    
    COPY incubator/ ./
    
    RUN CGO_ENABLED=0 GOOS=linux go build -o bin/content-automation-engine ./cmd/main.go
    
    # ---- Stage 2: Runner ----
    FROM scratch

    # TLS + SRV DNS: scratch has neither a CA bundle nor nsswitch, so mongodb+srv
    # and Reddit HTTPS fail certificate verification / hostname lookup otherwise.
    COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
    COPY --from=builder /etc/nsswitch.conf /etc/nsswitch.conf
    COPY --from=builder /build/bin/content-automation-engine /content-automation-engine

    ENV SSL_CERT_FILE=/etc/ssl/certs/ca-certificates.crt
    ENV SSL_CERT_DIR=/etc/ssl/certs
    
    EXPOSE 8000
    
    CMD ["/content-automation-engine"]