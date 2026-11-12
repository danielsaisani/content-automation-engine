# Content Automation Engine V2 (from Tiktok Incubator Project)


## Architecture
- Event Driven Modular Monolith
- Native Go concurrency and event buses
- Written In Golang
- Events notify that something has happened; consumers query the producer's API for details
- Inter-service transport (e.g. gRPC) is TBD; module contracts stay Go interfaces until then
- Each microservice owns it's contracts for other services to communicate with it
- One deployment process with all services in the backend