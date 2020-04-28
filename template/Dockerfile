FROM jingweno/heroku-editor:20 as builder

ARG HEROKU_USER
ARG HEROKU_API_KEY
ARG HEROKU_APP

COPY bin/heroku-git-clone /usr/local/bin/heroku-git-clone
RUN chmod +x /usr/local/bin/heroku-git-clone

RUN HEROKU_USER=${HEROKU_USER} HEROKU_API_KEY=${HEROKU_API_KEY} HEROKU_APP=${HEROKU_APP} /usr/local/bin/heroku-git-clone

FROM jingweno/heroku-editor:20

# Copy project source
COPY --from=builder /home/heroku/project/ /home/heroku/project/
