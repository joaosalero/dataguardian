.PHONY: setup dev test e2e auto go-test go-build

setup:
	./scripts/install.sh

dev:
	./start.sh manual

test:
	./run-tests.sh

e2e:
	./start.sh auto

auto:
	./start.sh auto

go-test:
	cd backend-go && go test ./...

go-build:
	./scripts/build_go.sh
