FROM heroku/heroku:20

# Set up `code-server`
RUN mkdir /usr/local/lib/code-server && \
      cd /usr/local/lib/code-server && \
      curl -sL https://github.com/cdr/code-server/releases/download/3.1.1/code-server-3.1.1-linux-x86_64.tar.gz | tar -xz --strip-components=1 && \
      ln -s /usr/local/lib/code-server/code-server /usr/local/bin/code-server

# TODO: install other languages
RUN mkdir /usr/local/lib/go && \
      cd /usr/local/lib/go && \
      curl -sL https://dl.google.com/go/go1.14.2.linux-amd64.tar.gz | tar -xz --strip-components=1 && \
      ln -s /usr/local/lib/go/bin/* /usr/local/bin
# TODO: install go tooling

RUN useradd -m heroku
USER heroku
WORKDIR /home/heroku

RUN mkdir -p /home/heroku/project

# TODO: add default setting
# TODO: download published exntesion from https://marketplace.visualstudio.com/_apis/public/gallery/publishers/jingweno/vsextensions/codeface/0.0.1/vspackage
COPY extensions/codeface-0.0.1.vsix /home/heroku/codeface.vsix
run /usr/local/bin/code-server --install-extension /home/heroku/codeface.vsix && rm -rf /home/heroku/codeface.vsix

# {
#    "go.formatTool": "goimports",
#        "go.useLanguageServer": true,
#            "terminal.integrated.shell.linux": "/bin/bash"
# }

#  gopl
#  gocode
#  gopkgs
#  go-outline
#  gocode-gomod
#  godef
#  goreturns
#  golint

# Install Go extension
# TODO: install other extensions
RUN /usr/local/bin/code-server --install-extension ms-vscode.go

COPY start-code-server /usr/local/bin/start-code-server

ENTRYPOINT /usr/local/bin/start-code-server
