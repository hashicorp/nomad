import { default as ApplicationAdapter, namespace } from './application';
import classic from 'ember-classic-decorator';

@classic
export default class PolicyAdapter extends ApplicationAdapter {
  namespace = namespace + '/acl';

  urlForCreateRecord(modelName, model) {
    return this.urlForUpdateRecord(model.attr('name'), 'policy');
  }

  urlForDeleteRecord(id, modelName, snapshot) {
    return this.urlForUpdateRecord(id, 'policy');
  }
}
