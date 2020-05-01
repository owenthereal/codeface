FROM heroku/heroku:20-build

RUN useradd -m -s /usr/bin/bash dyno
USER dyno
WORKDIR /home/dyno

ENV PATH /home/dyno/.heroku/bin:/home/dyno/go/bin:$PATH

COPY setup /home/dyno/setup
# TODO: download published exntesion from https://marketplace.visualstudio.com/_apis/public/gallery/publishers/jingweno/vsextensions/codeface/0.0.1/vspackage
COPY --chown=dyno extensions/codeface-0.0.1.vsix /home/dyno/codeface.vsix
RUN /home/dyno/setup && rm -rf /home/dyno/setup

COPY --chown=dyno settings.json /home/dyno/.local/share/code-server/User/settings.json
COPY --chown=dyno start-code-server /home/dyno/.heroku/bin/start-code-server
ENTRYPOINT start-code-server
