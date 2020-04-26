FROM codercom/code-server:latest

WORKDIR /home/coder

ADD bin/start-code-server /usr/local/bin/start-code-server

RUN curl -s https://cli-assets.heroku.com/install.sh | sh

ENTRYPOINT []
CMD /usr/local/bin/start-code-server
