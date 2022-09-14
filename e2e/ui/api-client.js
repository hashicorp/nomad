import axios from "axios";
import https from "https";
import fs from "fs";

export let NOMAD_PROXY_ADDR = process.env.NOMAD_PROXY_ADDR;
export let NOMAD_ADDR = process.env.NOMAD_ADDR;
export let NOMAD_TOKEN = process.env.NOMAD_TOKEN;
export let NOMAD_CLIENT_CERT = process.env.NOMAD_CLIENT_CERT;
export let NOMAD_CLIENT_KEY = process.env.NOMAD_CLIENT_KEY;
export let NOMAD_CACERT = process.env.NOMAD_CACERT;
export let NOMAD_TLS_SERVER_NAME = process.env.NOMAD_TLS_SERVER_NAME;

if (NOMAD_ADDR == undefined || NOMAD_ADDR == "") {
  NOMAD_ADDR = "http://localhost:4646";
}

if (NOMAD_PROXY_ADDR == undefined || NOMAD_PROXY_ADDR == "") {
  NOMAD_PROXY_ADDR = NOMAD_ADDR;
}

export const client = async (
  path,
  { data, method, headers: customHeaders, ...customConfig } = {},
  parameters
) => {
  const url = `${NOMAD_ADDR}/v1`.concat(path);

  let httpsAgent = new https.Agent({keepAlive: true});

  if (NOMAD_CLIENT_CERT !== "") {
    httpsAgent = new https.Agent({
      keepAlive: true,
      cert: fs.readFileSync(NOMAD_CLIENT_CERT),
      key: fs.readFileSync(NOMAD_CLIENT_KEY),
      ca: fs.readFileSync(NOMAD_CACERT),
    });
  }

  method = !!method ? method : data ? "post" : "get";

  const config = {
    url: parameters ? url.concat(`?${parameters.join('&')}`) : url,
    method,
    data: data ? data : undefined,
    headers: {
      "content-type": "application/json",
      "X-Nomad-Token": NOMAD_TOKEN,
      ...customHeaders,
    },
    httpsAgent,
    ...customConfig,
  };

  return axios(config).then(
    (res) => {
      if (res.statusText === "OK") {
        return res;
      }
    },
    (reason) => {
      throw new Error(reason.response.data);
    }
  );
};
