import ApplicationAdapter from './application';

export default class EvaluationAdapater extends ApplicationAdapter {
  handleResponse(_status, headers) {
    const result = super.handleResponse(...arguments);
    result.meta = { nextToken: headers['x-nomad-nexttoken'] };
    return result;
  }
}
