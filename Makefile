BINARY = thalassa-cloud-controller-manager
GOARCH = amd64

IMAGE 		?=registry.github.com/thalassa-cloud/thalassa-cloud-controller-manager
VERSION		?=local
COMMIT		?=$(shell git rev-parse HEAD)
BUILD_DATE	?=$(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
BUILD_BY 	?=make

PKG_LIST := $(shell go list ./... | grep -v /vendor/)

LDFLAGS = -X main.version=${VERSION} -X main.commit=${COMMIT} -X main.date=${BUILD_DATE} -X main.builtBy="${BUILD_BY}"

compile: build

linux:
	CGO_ENABLED=0 GOOS=linux GOARCH=${GOARCH} go build  -ldflags "${LDFLAGS}" -o ./bin/${BINARY}_linux_${GOARCH}/${BINARY} ./cmd/thalassa-cloud-controller-manager ;

darwin:
	CGO_ENABLED=0 GOOS=darwin GOARCH=${GOARCH} go build -ldflags "${LDFLAGS}" -o ./bin/${BINARY}_darwin_${GOARCH}/${BINARY} ./cmd/thalassa-cloud-controller-manager ;

build:
	CGO_ENABLED=0 go build -ldflags "${LDFLAGS}" -o ./bin/${BINARY} ./cmd/thalassa-cloud-controller-manager ;

snapshot:
	goreleaser release --clean --snapshot --skip=validate

lint: ## Lint the files
	@golint -set_exit_status ${PKG_LIST}

test: ## Run unittests
	@go test -short ${PKG_LIST}

race: ## Run data race detector
	@go test -race -short ${PKG_LIST}

msan: ## Run memory sanitizer
	@go test -msan -short ${PKG_LIST}

fmt:
	@go fmt ${PKG_LIST};

docker: linux
	docker build -t ${IMAGE}:${VERSION}${BRANCH} .

compose-down:
	docker compose down -v

compose:
	docker compose up -d database nats-server
	sleep 5
	./scripts/create-streams.sh

clean:
	-rm -f bin/${BINARY}-* bin/${BINARY}

review:
	reviewdog -diff="git diff FETCH_HEAD" -tee

swagger:
	swag init --parseDependency --parseInternal

.PHONY: link linux darwin windows test vet fmt clean

	
