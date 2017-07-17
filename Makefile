OBJ=s3sync
PWD=$(shell pwd)
BUILD_DIR=$(PWD)/objs
SOURCES=*.go


.PHONY: default
default: help


# help 提取注释作为帮助信息
help:                              ## Show this help.
	@fgrep -h "##" $(MAKEFILE_LIST) | fgrep -v fgrep | sed -e 's/\\$$//' | sed -e 's/##//'



.PHONY: clean
clean:
	rm -rf ${BUILD_DIR}


.PHONY: run-dev
run-dev:
	go run ${SOURCES} -env=dev

.PHONY: run-dev-prod
run-dev-prod:
	go run ${SOURCES} -env=dev-prod


.PHONY: build-linux
build-linux:
	GOOS=linux GOARCH=amd64 go build -o ${BUILD_DIR}/${OBJ} ${SOURCES}
