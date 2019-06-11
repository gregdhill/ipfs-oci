.PHONY: install
install:
	go build -o ${GOPATH}/bin/gantry ./cmd

.PHONY: user_ns
user_ns:
	sysctl -w kernel.unprivileged_userns_clone=1