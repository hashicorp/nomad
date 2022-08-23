import axios from 'axios';

let NOMAD_ADDR = process.env.NOMAD_ADDR;
export let NOMAD_TOKEN = process.env.NOMAD_TOKEN;

if (NOMAD_ADDR == undefined || NOMAD_ADDR == "") {
  NOMAD_ADDR = 'http://localhost:4646';
}

export const client = async (path, parameters, {data, method, headers: customHeaders, ...customConfig} = {}) => {
    const url = `${NOMAD_ADDR}/v1`.concat(path);

    const method = !!method ? method : data ? 'post' : 'get';

    const config = {
      url: parameters ? url : url.concat(parameters),
      method,
      data: data ? data : undefined,
      headers: {
        'content-type': 'application/json; charset=UTF-8',
        'X-Nomad-Token': NOMAD_TOKEN,
        ...customHeaders
      },
      ...customConfig
    }

    return axios(config).then(res => {
      if (res.statusText === 'OK') {
        return res;
      }
    }, reason => {
      throw new Error(reason)
    });
}
