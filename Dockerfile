FROM node:24-bookworm AS web
WORKDIR /app
ENV CI=true
RUN corepack enable && corepack prepare pnpm@11.8.0 --activate
COPY package.json pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile
COPY index.html tsconfig.json vite.config.ts ./
COPY src ./src
RUN pnpm build:web

FROM golang:1.25-bookworm AS go
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY migrations ./migrations
COPY *.go ./
COPY --from=web /app/dist ./dist
RUN CGO_ENABLED=0 go test ./...
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/plantingtulips .

FROM gcr.io/distroless/base-debian12:nonroot
WORKDIR /app
COPY --from=go /out/plantingtulips /app/plantingtulips
ENV ADDR=:8080
EXPOSE 8080
ENTRYPOINT ["/app/plantingtulips"]
