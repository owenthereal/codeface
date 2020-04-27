FROM heroku/heroku:20-build as builder

RUN curl -sL https://cli-assets.heroku.com/install.sh | sh

RUN mkdir /usr/local/lib/code-server && \
      cd /usr/local/lib/code-server && \
      curl -sL https://github.com/cdr/code-server/releases/download/3.1.1/code-server-3.1.1-linux-x86_64.tar.gz | tar -xz --strip-components=1

ARG HEROKU_USER
ARG HEROKU_API_KEY
ARG HEROKU_APP

WORKDIR /home/heroku

COPY bin/heroku-git-clone /home/heroku/bin/heroku-git-clone
RUN chmod +x /home/heroku/bin/heroku-git-clone

RUN HEROKU_USER=${HEROKU_USER} HEROKU_API_KEY=${HEROKU_API_KEY} HEROKU_APP=${HEROKU_APP} /home/heroku/bin/heroku-git-clone

FROM heroku/heroku:20-build

WORKDIR /home/heroku

# Copy `code-server`
COPY --from=builder /usr/local/lib/code-server/ /usr/local/lib/code-server/
RUN ln -s /usr/local/lib/code-server/code-server /usr/local/bin/code-server

# Copy `heroku`
COPY --from=builder /usr/local/lib/heroku/ /usr/local/lib/heroku/
RUN ln -s /usr/local/lib/heroku/bin/heroku /usr/local/bin/heroku

# Copy project source
COPY --from=builder /home/heroku/project/ /home/heroku/project/

# Copy start up script
COPY bin/start-code-server /usr/local/bin/start-code-server
RUN chmod +x /usr/local/bin/start-code-server

ENTRYPOINT /usr/local/bin/start-code-server
