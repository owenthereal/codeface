FROM heroku/heroku:20

WORKDIR /home/heroku

# Set up `heroku`
RUN curl -sL https://cli-assets.heroku.com/install.sh | sh

# Set up `code-server`
RUN mkdir /usr/local/lib/code-server && \
      cd /usr/local/lib/code-server && \
      curl -sL https://github.com/cdr/code-server/releases/download/3.1.1/code-server-3.1.1-linux-x86_64.tar.gz | tar -xz --strip-components=1 && \
      ln -s /usr/local/lib/code-server/code-server /usr/local/bin/code-server

# Copy start up script
COPY bin/start-code-server /usr/local/bin/start-code-server
RUN chmod +x /usr/local/bin/start-code-server

ENTRYPOINT /usr/local/bin/start-code-server
