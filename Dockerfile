FROM node:24-alpine AS frontend
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci --no-audit --no-fund
COPY web/ ./
RUN npm run build

FROM golang:1.25-alpine AS backend
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY assets.go ./
COPY cmd/ ./cmd/
COPY internal/ ./internal/
COPY floorplans/ ./floorplans/
COPY --from=frontend /src/web/dist ./web/dist
RUN CGO_ENABLED=0 go build -buildvcs=false -trimpath -o /yanetviz ./cmd/yanetviz

FROM alpine:3.22
RUN apk add --no-cache ca-certificates && adduser -D -u 10001 yanetviz && mkdir /data && chown yanetviz /data
COPY --from=backend /yanetviz /usr/local/bin/yanetviz
USER yanetviz
EXPOSE 8080
VOLUME /data
ENTRYPOINT ["yanetviz"]
CMD ["-listen", "0.0.0.0:8080", "-data", "/data"]
