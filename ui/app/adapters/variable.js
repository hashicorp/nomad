import ApplicationAdapter from './application';
import { pluralize } from 'ember-inflector';
import classic from 'ember-classic-decorator';
import { ConflictError } from '@ember-data/adapter/error';

@classic
export default class VariableAdapter extends ApplicationAdapter {
  pathForType = () => 'var';

  // PUT instead of POST on create;
  // /v1/var instead of /v1/vars on create (urlForFindRecord)
  createRecord(_store, type, snapshot) {
    let data = this.serialize(snapshot);
    let baseUrl = this.buildURL(type.modelName, data.ID);
    const checkAndSetValue = snapshot?.attr('modifyIndex') || 0;
    return this.ajax(`${baseUrl}?cas=${checkAndSetValue}`, 'PUT', { data });
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
    if (snapshot?.adapterOptions?.overwrite) {
      return `${baseUrl}`;
    } else {
      const checkAndSetValue = snapshot?.attr('modifyIndex') || 0;
      return `${baseUrl}?cas=${checkAndSetValue}`;
    }
  }

  urlForDeleteRecord(identifier, modelName, snapshot) {
    const { namespace, id } = _extractIDAndNamespace(identifier, snapshot);
    const baseUrl = this.buildURL(modelName, id);
    return `${baseUrl}?namespace=${namespace}`;
  }

  handleResponse(status, _, payload) {
    if (status === 409) {
      return new ConflictError([
        { detail: _normalizeConflictErrorObject(payload), status: 409 },
      ]);
    }
    return super.handleResponse(...arguments);
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

function _normalizeConflictErrorObject(conflictingVariable) {
  return {
    modifyTime: Math.floor(conflictingVariable.ModifyTime / 1000000),
    items: conflictingVariable.Items,
  };
}
