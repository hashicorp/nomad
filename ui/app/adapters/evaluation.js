import ApplicationAdapter from './application';
import classic from 'ember-classic-decorator';

@classic
export default class EvaluationAdapter extends ApplicationAdapter {
  handleResponse(_status, headers) {
    const result = super.handleResponse(...arguments);
    result.meta = { nextToken: headers['x-nomad-nexttoken'] };
    return result;
  }
}
