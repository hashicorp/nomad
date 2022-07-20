import ApplicationAdapter from './application';
import { pluralize } from 'ember-inflector';
import classic from 'ember-classic-decorator';

@classic
export default class VariableAdapter extends ApplicationAdapter {
  pathForType = () => 'var';

  // PUT instead of POST on create;
  // /v1/var instead of /v1/vars on create (urlForFindRecord)
  createRecord(_store, _type, snapshot) {
    let data = this.serialize(snapshot);
    return this.ajax(
      this.urlForFindRecord(snapshot.id, snapshot.modelName),
      'PUT',
      { data }
    );
  }

  urlForFindAll(modelName) {
    let baseUrl = this.buildURL(modelName);
    return pluralize(baseUrl);
  }

  urlForQuery(_query, modelName) {
    let baseUrl = this.buildURL(modelName);
    return pluralize(baseUrl);
  }

  urlForFindRecord(id, modelName, snapshot) {
    const namespace = snapshot?.attr('namespace') || 'default';

    let baseUrl = this.buildURL(modelName, id, snapshot);
    return `${baseUrl}?namespace=${namespace}`;
  }

  urlForUpdateRecord(id, modelName) {
    return this.buildURL(modelName, id);
  }

  urlForDeleteRecord(id, modelName, snapshot) {
    const namespace = snapshot?.attr('namespace') || 'default';

    const baseUrl = this.buildURL(modelName, id);
    return `${baseUrl}?namespace=${namespace}`;
  }
}
