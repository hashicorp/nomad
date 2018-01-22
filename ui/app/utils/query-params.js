// simpler/reusable way to create an ember 'query params' object
// currently used for easily adding query params to link-to params
export default function queryParams(obj) {
  return {
    isQueryParams: true,
    values: obj,
  };
}
