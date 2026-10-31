BIN := bin/herelies

.PHONY: run build web test check live eval e2e clean

run: build
	./$(BIN)

build: web
	go build -o $(BIN) ./cmd/herelies

web: web/node_modules
	cd web && npm run build
	@touch web/dist/.keep

web/node_modules: web/package-lock.json
	cd web && npm ci
	@touch web/node_modules

test:
	@out="$$(gofmt -l cmd internal web/embed.go)"; if [ -n "$$out" ]; then echo "gofmt needed:"; echo "$$out"; exit 1; fi
	go vet ./...
	go test ./...

# The real pipeline on every photo in testdata/photos with the real model.
live:
	go test -tags live -run TestLive -count=1 -timeout 60m -v ./internal/obit

# Builds eval/results.md and eval/results.csv from the last live run and the photo review.
eval:
	go run ./cmd/evalreport

# Browser tests against the real server and the real model.
e2e: build
	cd e2e && npm ci && npx playwright test

clean:
	rm -rf bin web/dist/assets web/dist/index.html web/dist/favicon.svg e2e/out
