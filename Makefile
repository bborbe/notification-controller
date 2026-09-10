include Makefile.variables
include Makefile.precommit
include Makefile.docker

SERVICE = notification-controller

.PHONY: run
run:
	@go run -mod=mod . -listen="localhost:8080" -datadir="$(DATADIR)" -kafka-brokers="$(KAFKA_BROKERS)" -branch="$(BRANCH)" -v=2
