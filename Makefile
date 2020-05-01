.PHONY: build
build:
	go build -o bin/cf-client ./cmd/cf-client
	go build -o bin/cf-server ./cmd/cf-server

.PHONY: base-image
base-image: vscode-ext
	cd ./base-image && docker build -t jingweno/heroku-editor:20 . && docker push jingweno/heroku-editor:20

.PHONY: vscode-ext
vscode-ext:
	cd ./vscode && vsce package -o ../base-image/extensions
