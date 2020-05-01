.PHONY: build
build:
	GOOS=js GOARCH=wasm go build -o web/assets/main.wasm ./web/...
	go-bindata -o server/bindata.go -pkg server -fs -prefix "web/assets" ./web/assets
	go build -o bin/cf ./cmd/cf

.PHONY: base-image
base-image: vscode-ext
	cd ./base-image && docker build -t jingweno/heroku-editor:20 . && docker push jingweno/heroku-editor:20

.PHONY: vscode-ext
vscode-ext:
	cd ./vscode-ext && vsce package -o ../base-image/extensions
