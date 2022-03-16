import { get } from '@ember/object';
import ApplicationAdapter from './application';

export default class EvaluationAdapter extends ApplicationAdapter {
  handleResponse(_status, headers) {
    const result = super.handleResponse(...arguments);
    result.meta = { nextToken: headers['x-nomad-nexttoken'] };
    return result;
  }

  urlForFindRecord(id, modelName, snapshot) {
    let url = super.urlForFindRecord(...arguments);
    return `${url}?related=true`;
  }
}
