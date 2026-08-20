docker-build:
	docker build . -t oamdev/cluster-register:v1.0

.PHONY: e2e-up e2e-down e2e

e2e-up:
	test/e2e/scripts/up.sh

e2e-down:
	test/e2e/scripts/down.sh

e2e:
	test/e2e/scripts/run.sh