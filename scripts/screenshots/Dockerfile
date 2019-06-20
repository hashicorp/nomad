FROM buildkite/puppeteer:v1.15.0

COPY src .

ENV EMBER_HOST=http://host.docker.internal:4200

RUN npm install

CMD [ "node", "index.js" ]

