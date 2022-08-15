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
      this.urlForFindRecord(data.Path, snapshot.modelName),
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

  urlForFindRecord(identifier, modelName, snapshot) {
    const { namespace, id } = _extractIDAndNamespace(identifier, snapshot);
    let baseUrl = this.buildURL(modelName, id);
    return `${baseUrl}?namespace=${namespace}`;
  }

  urlForUpdateRecord(identifier, modelName, snapshot) {
    const { id } = _extractIDAndNamespace(identifier, snapshot);
    let baseUrl = this.buildURL(modelName, id);
    return `${baseUrl}`;
  }

  urlForDeleteRecord(identifier, modelName, snapshot) {
    const { namespace, id } = _extractIDAndNamespace(identifier, snapshot);
    const baseUrl = this.buildURL(modelName, id);
    return `${baseUrl}?namespace=${namespace}`;
  }
}

function _extractIDAndNamespace(identifier, snapshot) {
  const namespace = snapshot?.attr('namespace') || 'default';
  const id = snapshot?.attr('path') || identifier;
  return {
    namespace,
    id,
  };
}
