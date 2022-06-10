import ApplicationAdapter from './application';
import classic from 'ember-classic-decorator';

@classic
export default class EvaluationAdapter extends ApplicationAdapter {
  handleResponse(_status, headers) {
    const result = super.handleResponse(...arguments);
    result.meta = { nextToken: headers['x-nomad-nexttoken'] };
    return result;
  }

  urlForFindRecord(_id, _modelName, snapshot) {
    const url = super.urlForFindRecord(...arguments);

    if (snapshot.adapterOptions?.related) {
      return `${url}?related=true`;
    }
    return url;
  }
}
