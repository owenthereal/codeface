FROM heroku/heroku:20-build

WORKDIR /home/coder

RUN mkdir /usr/local/lib/code-server && \
      cd /usr/local/lib/code-server && \
      curl -sL https://github.com/cdr/code-server/releases/download/3.1.1/code-server-3.1.1-linux-x86_64.tar.gz | tar -xz --strip-components=1 && \
      ln -s /usr/local/lib/code-server/code-server /usr/local/bin/code-server

RUN curl -sL https://cli-assets.heroku.com/install.sh | sh

ADD bin/start-code-server /usr/local/bin/start-code-server

ENTRYPOINT []
CMD /usr/local/bin/start-code-server
