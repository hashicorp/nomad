import axios from "axios";

export let NOMAD_ADDR = process.env.NOMAD_ADDR;
export let NOMAD_TOKEN = process.env.NOMAD_TOKEN;

if (NOMAD_ADDR == undefined || NOMAD_ADDR == "") {
  NOMAD_ADDR = "http://localhost:4646";
}

export const client = async (
  path,
  { data, method, headers: customHeaders, ...customConfig } = {},
  parameters
) => {
  const url = `${NOMAD_ADDR}/v1`.concat(path);

  method = !!method ? method : data ? "post" : "get";

  const config = {
    url: parameters ? url.concat(parameters) : url,
    method,
    data: data ? data : undefined,
    headers: {
      "content-type": "application/json",
      "X-Nomad-Token": NOMAD_TOKEN,
      ...customHeaders,
    },
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
